package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"backup-agent/internal/report"
)

const (
	defaultInterval = 10 * time.Minute
	businessWait    = 6 * time.Hour
	maxBackoff      = time.Hour
	batchSize       = 50
	maxRejectedKept = 20
)

// Info describe este equipo y este agente.
type Info struct {
	Hostname      string
	OS            string
	AgentVersion  string
	ResticVersion string
}

// Options son las dependencias del envío. Todo lo que es del agente entra por aquí.
type Options struct {
	Store      *Store
	DataDir    string // donde puede estar alta-backupamg.json
	ReportsDir string
	// ServerURL y SendPaths son ajustes de la configuración; se leen en cada vuelta.
	ServerURL func() string
	SendPaths func() bool
	// SetServerURL guarda en la configuración la dirección que trae el fichero de alta.
	SetServerURL func(string) error
	Describe     func() Info
	// Heartbeat construye el latido a partir del estado actual; recibe los informes pendientes.
	Heartbeat func(pending int) Heartbeat
	// Protect y Open cifran y descifran la clave del equipo (DPAPI).
	Protect    func(string) (string, error)
	Open       func(string) (string, error)
	MachineUID func() (string, error)
	Log        *slog.Logger
	Now        func() time.Time
}

// Sender envía latidos e informes. Corre en su propio hilo; nada de lo que pasa aquí afecta a las copias.
type Sender struct {
	o      Options
	kick   chan struct{}
	force  chan struct{}
	stepMu sync.Mutex
}

// NewSender prepara el envío.
func NewSender(o Options) *Sender {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	return &Sender{o: o, kick: make(chan struct{}, 1), force: make(chan struct{}, 1)}
}

// SetLogger fija dónde se anota lo que pasa. Se llama antes de Run.
func (s *Sender) SetLogger(l *slog.Logger) {
	if l != nil {
		s.o.Log = l
	}
}

// Kick avisa de que hay algo nuevo que enviar (por ejemplo, un informe recién escrito).
func (s *Sender) Kick() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// SyncNow pide enviar ya, aunque toque esperar (botón «Enviar ahora»).
func (s *Sender) SyncNow() {
	select {
	case s.force <- struct{}{}:
	default:
	}
}

// Run es el bucle del hilo de envío. Termina cuando ctx se cancela.
func (s *Sender) Run(ctx context.Context) {
	timer := time.NewTimer(5 * time.Second) // deja que el agente arranque antes del primer latido
	defer timer.Stop()
	for {
		force, kicked := false, false
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		case <-s.kick:
			kicked = true
		case <-s.force:
			force = true
		}
		s.Step(ctx, force, kicked)
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(s.nextWait())
	}
}

// nextWait es cuánto esperar hasta la próxima vuelta (como mucho, el intervalo del latido).
func (s *Sender) nextWait() time.Duration {
	d := s.o.Store.Snapshot()
	wait := interval(d)
	if next, err := time.Parse(time.RFC3339, d.NextAttempt); err == nil {
		if until := next.Sub(s.o.Now()); until > 0 && until < wait {
			wait = until
		}
	}
	if wait < 5*time.Second {
		wait = 5 * time.Second
	}
	return wait
}

func interval(d Data) time.Duration {
	if d.IntervalSeconds >= 60 {
		return time.Duration(d.IntervalSeconds) * time.Second
	}
	return defaultInterval
}

// Step hace una vuelta: alta automática si hay fichero, latido e informes pendientes.
func (s *Sender) Step(ctx context.Context, force, kicked bool) {
	s.stepMu.Lock()
	defer s.stepMu.Unlock()
	defer func() {
		if r := recover(); r != nil { // nada de esto puede tumbar el agente
			s.o.Log.Error("Fallo inesperado en el envío a la plataforma", "error", r)
		}
	}()
	if strings.TrimSpace(s.o.ServerURL()) == "" {
		// Sin dirección solo puede haber alta automática con el fichero (trae la dirección).
		s.autoEnroll(ctx)
		return
	}
	d := s.o.Store.Snapshot()
	if d.KeyEncrypted == "" {
		s.autoEnroll(ctx)
		return
	}
	if d.ServerURL != "" && !sameURL(d.ServerURL, s.o.ServerURL()) {
		return // la dirección cambió: hay que dar de alta de nuevo (lo gestiona Forget/Enroll)
	}
	now := s.o.Now()
	due := force
	if next, err := time.Parse(time.RFC3339, d.NextAttempt); err != nil || !now.Before(next) {
		due = true
	}
	if kicked && d.State == StateConnected && d.Failures == 0 && len(d.Queue) > 0 {
		due = true
	}
	if !due {
		return
	}
	s.sync(ctx, d)
}

func sameURL(a, b string) bool {
	norm := func(u string) string {
		c, err := normalizeBase(u)
		if err != nil {
			return strings.ToLower(strings.TrimRight(strings.TrimSpace(u), "/"))
		}
		return strings.ToLower(c)
	}
	return norm(a) == norm(b)
}

// sync envía el latido y, si la plataforma responde bien, los informes pendientes.
func (s *Sender) sync(ctx context.Context, d Data) {
	key, err := s.o.Open(d.KeyEncrypted)
	if err != nil {
		s.o.Log.Error("No se pudo leer la clave del equipo para la plataforma")
		s.setState(StateNeedsEnroll, "", "")
		return
	}
	client, err := NewClient(s.o.ServerURL())
	if err != nil {
		s.setState(StateNotConfigured, "", err.Error())
		return
	}
	hb := s.o.Heartbeat(len(d.Queue))
	resp, err := client.SendHeartbeat(ctx, key, hb)
	if err != nil {
		s.handleError(err, "latido")
		return
	}
	_ = s.o.Store.Update(func(x *Data) {
		x.State, x.StateCode, x.StateMessage = StateConnected, "", ""
		x.StateAt = s.o.Now().Format(time.RFC3339)
		x.Failures = 0
		x.LastHeartbeatAt = s.o.Now().Format(time.RFC3339)
		if resp.License.Status != "" || resp.License.MaxMachines != 0 {
			x.License = resp.License
		}
		if resp.HeartbeatIntervalSecs >= 60 {
			x.IntervalSeconds = resp.HeartbeatIntervalSecs
		}
		x.MinAgentVersion, x.RecommendedAgentVersion = resp.MinAgentVersion, resp.RecommendedAgentVersion
		x.NextAttempt = s.o.Now().Add(interval(*x)).Format(time.RFC3339)
	})
	s.sendReports(ctx, client, key)
}

func (s *Sender) sendReports(ctx context.Context, client *Client, key string) {
	machineUID, _ := s.o.MachineUID()
	for ctx.Err() == nil {
		d := s.o.Store.Snapshot()
		if len(d.Queue) == 0 {
			return
		}
		type sent struct {
			file string
			uid  string
		}
		var batch []sent
		var payload []json.RawMessage
		var dropped []string
		for _, item := range d.Queue {
			if len(batch) >= batchSize {
				break
			}
			raw, err := os.ReadFile(filepath.Join(s.o.ReportsDir, item.File))
			if err != nil {
				dropped = append(dropped, item.File) // ya no existe (limpieza de 90 días)
				continue
			}
			uid := legacyOrExisting(raw, machineUID, item.File)
			prepared, err := PrepareReport(raw, uid, s.o.SendPaths())
			if err != nil {
				s.o.Log.Warn("Informe no válido: no se envía", "file", item.File)
				dropped = append(dropped, item.File)
				continue
			}
			batch = append(batch, sent{file: item.File, uid: uid})
			payload = append(payload, prepared)
		}
		if len(batch) == 0 {
			s.ack(dropped, nil)
			if len(dropped) == 0 {
				return
			}
			continue
		}
		resp, err := client.SendReports(ctx, key, payload)
		if err != nil {
			s.handleError(err, "informes")
			return
		}
		byUID := map[string]ReportResult{}
		for _, r := range resp.Results {
			byUID[r.ReportUID] = r
		}
		var done []string
		var rejected []string
		storedCount, duplicateCount := 0, 0
		for _, b := range batch {
			r, ok := byUID[b.uid]
			if !ok {
				continue // sin resultado: se reintenta
			}
			switch r.Result {
			case "stored", "duplicate":
				if r.Result == "stored" {
					storedCount++
				} else {
					duplicateCount++
				}
				done = append(done, b.file)
			case "rejected":
				done = append(done, b.file)
				rejected = append(rejected, b.file+" ("+r.Code+")")
			}
		}
		s.ack(append(done, dropped...), rejected)
		_ = s.o.Store.Update(func(x *Data) { x.LastReportsAt = s.o.Now().Format(time.RFC3339) })
		s.o.Log.Info("Informes enviados a la plataforma", "count", len(done), "stored", storedCount, "duplicate", duplicateCount, "rejected", len(rejected))
		if len(done) == 0 {
			return // la plataforma no confirmó ninguno: no se insiste en este ciclo
		}
	}
}

func legacyOrExisting(raw []byte, machineUID, file string) string {
	var head struct {
		UID string `json:"report_uid"`
	}
	if json.Unmarshal(raw, &head) == nil && strings.TrimSpace(head.UID) != "" {
		return head.UID
	}
	return report.LegacyUID(machineUID, file)
}

func (s *Sender) ack(files []string, rejected []string) {
	if len(files) == 0 && len(rejected) == 0 {
		return
	}
	drop := map[string]bool{}
	for _, f := range files {
		drop[f] = true
	}
	_ = s.o.Store.Update(func(x *Data) {
		kept := x.Queue[:0]
		for _, item := range x.Queue {
			if !drop[item.File] {
				kept = append(kept, item)
			}
		}
		x.Queue = kept
		x.Rejected = append(x.Rejected, rejected...)
		if over := len(x.Rejected) - maxRejectedKept; over > 0 {
			x.Rejected = x.Rejected[over:]
		}
	})
}

// handleError aplica las reglas del contrato (6.6): una decisión de la plataforma se muestra y se
// vuelve a mirar dentro de unas horas; un fallo de red o del servidor se reintenta con espera creciente.
func (s *Sender) handleError(err error, what string) {
	api := asAPIError(err)
	switch {
	case api.IsBusiness():
		state := stateForCode(api.Code)
		s.o.Log.Warn("La plataforma ha respondido; las copias siguen", "code", api.Code, "step", what)
		_ = s.o.Store.Update(func(x *Data) {
			x.State, x.StateCode, x.StateMessage = state, api.Code, api.Message
			x.StateAt = s.o.Now().Format(time.RFC3339)
			x.NextAttempt = s.o.Now().Add(businessWait).Format(time.RFC3339)
		})
	default:
		_ = s.o.Store.Update(func(x *Data) {
			x.Failures++
			wait := backoff(x.Failures)
			if api.RetryAfter > wait {
				wait = api.RetryAfter
			}
			x.State, x.StateCode, x.StateMessage = StateOffline, api.Code, ""
			x.StateAt = s.o.Now().Format(time.RFC3339)
			x.NextAttempt = s.o.Now().Add(wait).Format(time.RFC3339)
		})
		s.o.Log.Info("Sin conexión con la plataforma; se reintentará", "step", what, "status", api.HTTPStatus)
	}
}

func stateForCode(code string) string {
	switch code {
	case CodeLicenseExpired:
		return StateLicenseExpired
	case CodeLicenseBlocked, CodeLicenseNotStarted:
		return StateLicenseBlocked
	case CodeSeatLimit:
		return StateSeatLimit
	case CodeMachineRevoked:
		return StateRevoked
	case CodeInvalidKey:
		return StateNeedsEnroll
	}
	return StateOffline
}

// backoff es la espera tras n fallos seguidos: 1 min, 2, 4… hasta 1 hora, con algo de azar.
func backoff(n int) time.Duration {
	if n < 1 {
		n = 1
	}
	wait := time.Minute
	for i := 1; i < n && wait < maxBackoff; i++ {
		wait *= 2
	}
	if wait > maxBackoff {
		wait = maxBackoff
	}
	jitter := time.Duration(rand.Int64N(int64(wait/10) + 1))
	return wait + jitter
}

func (s *Sender) setState(state, code, message string) {
	_ = s.o.Store.Update(func(x *Data) {
		x.State, x.StateCode, x.StateMessage = state, code, message
		x.StateAt = s.o.Now().Format(time.RFC3339)
	})
}

// ---- Alta ----------------------------------------------------------------------------------

// Enroll da de alta el equipo con una clave de licencia (BKA-…) o un código de instalación (BKI-…).
// Si tiene éxito, guarda la clave del equipo cifrada y empieza a enviar. El error es un *APIError
// (con Code) para que la pantalla diga qué pasó; nunca detiene las copias.
func (s *Sender) Enroll(ctx context.Context, credential string) error {
	s.stepMu.Lock()
	defer s.stepMu.Unlock()
	return s.enroll(ctx, credential)
}

func (s *Sender) enroll(ctx context.Context, credential string) error {
	credential = strings.TrimSpace(credential)
	if credential == "" {
		return &APIError{Code: CodeInvalidRequest}
	}
	client, err := NewClient(s.o.ServerURL())
	if err != nil {
		return &APIError{Code: "bad_server_url", Message: err.Error(), cause: err}
	}
	uid, err := s.o.MachineUID()
	if err != nil {
		return &APIError{Code: "machine_uid", Message: err.Error(), cause: err}
	}
	info := s.o.Describe()
	req := EnrollRequest{
		MachineUID: uid, Hostname: info.Hostname, OS: info.OS, AgentVersion: info.AgentVersion,
		ResticVersion: info.ResticVersion, ClientTime: s.o.Now().UTC().Format(time.RFC3339), SchemaVersion: 1,
	}
	if IsInstallCode(credential) {
		req.InstallCode = credential
	} else {
		req.LicenseKey = credential
	}
	resp, err := client.Enroll(ctx, req)
	if err != nil {
		api := asAPIError(err)
		s.o.Log.Warn("No se pudo dar de alta el equipo en la plataforma", "code", api.Code, "status", api.HTTPStatus)
		if api.IsBusiness() {
			_ = s.o.Store.Update(func(x *Data) {
				if x.KeyEncrypted == "" {
					x.State, x.StateCode, x.StateMessage = StateNotEnrolled, api.Code, api.Message
					x.StateAt = s.o.Now().Format(time.RFC3339)
				}
			})
		}
		return err
	}
	encrypted, err := s.o.Protect(resp.MachineKey)
	if err != nil {
		return &APIError{Code: "protect", cause: errors.New("no se pudo guardar la clave del equipo")}
	}
	now := s.o.Now().Format(time.RFC3339)
	if err := s.o.Store.Update(func(x *Data) {
		x.KeyEncrypted = encrypted
		x.ServerURL = s.o.ServerURL()
		x.CompanyName = resp.Company.Name
		x.EnrolledAt = now
		x.License = resp.License
		x.State, x.StateCode, x.StateMessage, x.StateAt = StateConnected, "", "", now
		x.Failures = 0
		x.NextAttempt = now // enviar enseguida
		if resp.HeartbeatIntervalSecs >= 60 {
			x.IntervalSeconds = resp.HeartbeatIntervalSecs
		}
		x.MinAgentVersion, x.RecommendedAgentVersion = resp.MinAgentVersion, resp.RecommendedAgentVersion
	}); err != nil {
		// La clave ya está en la plataforma pero no se pudo guardar: se dice, y se vuelve a dar de alta.
		return &APIError{Code: "store", cause: err}
	}
	s.o.Log.Info("Equipo dado de alta en la plataforma", "result", resp.Code, "company", resp.Company.Name)
	s.Kick()
	return nil
}

// autoEnroll da de alta con alta-backupamg.json si el instalador lo dejó en la carpeta de datos.
// El fichero se borra cuando el alta se resuelve (éxito o error definitivo); si fue un fallo de
// red o un límite que puede cambiar, se conserva para volver a intentarlo.
func (s *Sender) autoEnroll(ctx context.Context) {
	path := filepath.Join(s.o.DataDir, AltaFileName)
	alta, err := ReadAlta(path)
	if err != nil {
		s.o.Log.Warn("El fichero de alta no se puede usar", "error", err.Error())
		_ = os.Remove(path)
		return
	}
	if alta == nil {
		return
	}
	// No se reintenta antes de la hora si ya falló hace poco.
	d := s.o.Store.Snapshot()
	if next, perr := time.Parse(time.RFC3339, d.NextAttempt); perr == nil && s.o.Now().Before(next) && d.KeyEncrypted == "" && d.State != "" {
		return
	}
	if current := strings.TrimSpace(s.o.ServerURL()); current == "" || !sameURL(current, alta.ServerURL) {
		if s.o.SetServerURL == nil {
			return
		}
		if err := s.o.SetServerURL(alta.ServerURL); err != nil {
			s.o.Log.Warn("No se pudo guardar la dirección del servidor del fichero de alta", "error", err.Error())
			return
		}
	}
	err = s.enroll(ctx, alta.InstallCode)
	if err == nil {
		_ = os.Remove(path)
		return
	}
	api := asAPIError(err)
	switch api.Code {
	case CodeInvalidInstallCode, CodeInstallRevoked, CodeInstallExpired, CodeInvalidLicense, CodeMachineRevoked, CodeInvalidRequest:
		_ = os.Remove(path) // error definitivo: el código ya no sirve
	}
	// Lo demás (red, límite, licencia) se conserva y se vuelve a mirar más tarde.
	_ = s.o.Store.Update(func(x *Data) {
		wait := businessWait
		if api.IsRetryable() {
			x.Failures++
			wait = backoff(x.Failures)
		}
		x.NextAttempt = s.o.Now().Add(wait).Format(time.RFC3339)
	})
}

// EnrollFromFile hace el alta con alta-backupamg.json si el instalador lo dejó en la carpeta de datos.
func (s *Sender) EnrollFromFile(ctx context.Context) {
	s.stepMu.Lock()
	defer s.stepMu.Unlock()
	s.autoEnroll(ctx)
}

// Forget borra la clave del equipo (por ejemplo, al cambiar la dirección del servidor). La cola se conserva.
func (s *Sender) Forget() error {
	return s.o.Store.Update(func(x *Data) {
		x.KeyEncrypted, x.CompanyName, x.EnrolledAt = "", "", ""
		x.License = LicenseInfo{}
		x.State, x.StateCode, x.StateMessage = "", "", ""
		x.Failures, x.NextAttempt, x.ServerURL = 0, "", ""
	})
}

// ---- Vista para el panel ------------------------------------------------------------------

// View es lo que ven el panel local y la bandeja.
type View struct {
	State        string      `json:"state"`
	Code         string      `json:"code,omitempty"`
	Message      string      `json:"message,omitempty"`
	ServerURL    string      `json:"server_url"`
	Enrolled     bool        `json:"enrolled"`
	Company      string      `json:"company,omitempty"`
	Pending      int         `json:"pending"`
	LastSyncAt   string      `json:"last_sync_at,omitempty"`
	NextAttempt  string      `json:"next_attempt,omitempty"`
	License      LicenseInfo `json:"license"`
	SendPaths    bool        `json:"send_paths"`
	UpdateAdvice string      `json:"update_advice,omitempty"`
	AltaFile     bool        `json:"alta_file,omitempty"`
}

// View resume el estado de la conexión.
func (s *Sender) View() View {
	d := s.o.Store.Snapshot()
	url := strings.TrimSpace(s.o.ServerURL())
	v := View{
		ServerURL: url, Enrolled: d.KeyEncrypted != "", Company: d.CompanyName, Pending: len(d.Queue),
		LastSyncAt: d.LastHeartbeatAt, NextAttempt: d.NextAttempt, License: d.License,
		SendPaths: s.o.SendPaths(), Code: d.StateCode, Message: d.StateMessage,
	}
	if _, err := os.Stat(filepath.Join(s.o.DataDir, AltaFileName)); err == nil {
		v.AltaFile = true
	}
	switch {
	case url == "" && !v.Enrolled:
		v.State = StateNotConfigured
	case !v.Enrolled:
		v.State = StateNotEnrolled
	case d.ServerURL != "" && !sameURL(d.ServerURL, url):
		v.State = StateNeedsEnroll
	case d.State == "":
		v.State = StateOffline
	default:
		v.State = d.State
	}
	return v
}
