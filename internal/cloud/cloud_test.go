package cloud

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---- servidor de pruebas que imita el contrato v1 ------------------------------------------

type fakeServer struct {
	t  *testing.T
	ts *httptest.Server

	mu         sync.Mutex
	down       bool   // responde 503
	enrollErr  string // código de negocio a devolver en enroll
	hbErr      string // código de negocio a devolver en heartbeat
	reportsErr string // código de negocio a devolver en reports
	stored     map[string]bool
	order      []string // report_uid en el orden en que se guardaron
	hbCount    int
	lastHB     map[string]any
	enrolls    []map[string]any
	rawReports []json.RawMessage
	rejectUID  string
	dropAnswer bool // guarda los informes pero «pierde» la respuesta (corte de red)
}

func newFakeServer(t *testing.T) *fakeServer {
	f := &fakeServer{t: t, stored: map[string]bool{}}
	f.ts = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.ts.Close)
	return f
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func businessStatus(code string) int {
	switch code {
	case "invalid_license", "invalid_install_code", "invalid_key", "machine_revoked":
		return 401
	case "seat_limit_reached":
		return 409
	}
	return 403
}

func (f *fakeServer) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		writeJSON(w, 503, map[string]any{"ok": false, "code": "maintenance"})
		return
	}
	body, _ := io.ReadAll(r.Body)
	switch strings.TrimPrefix(r.URL.Path, "/backup/v1/") {
	case "enroll":
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		f.enrolls = append(f.enrolls, req)
		if f.enrollErr != "" {
			writeJSON(w, businessStatus(f.enrollErr), map[string]any{"ok": false, "code": f.enrollErr, "message": "no"})
			return
		}
		writeJSON(w, 200, map[string]any{
			"ok": true, "code": "enrolled", "machine_key": "bkm_clave-secreta-de-prueba-0123456789",
			"company": map[string]any{"name": "Empresa X"}, "heartbeat_interval_seconds": 600,
			"license": map[string]any{"status": "active", "max_machines": 2, "used_machines": 1},
		})
	case "heartbeat":
		if r.Header.Get("Authorization") != "Bearer bkm_clave-secreta-de-prueba-0123456789" {
			writeJSON(w, 401, map[string]any{"ok": false, "code": "invalid_key"})
			return
		}
		if f.hbErr != "" {
			writeJSON(w, businessStatus(f.hbErr), map[string]any{"ok": false, "code": f.hbErr})
			return
		}
		f.hbCount++
		f.lastHB = map[string]any{}
		_ = json.Unmarshal(body, &f.lastHB)
		writeJSON(w, 200, map[string]any{"ok": true, "code": "ok", "heartbeat_interval_seconds": 600,
			"license": map[string]any{"status": "active", "max_machines": 2, "used_machines": 1}})
	case "reports":
		if f.reportsErr != "" {
			writeJSON(w, businessStatus(f.reportsErr), map[string]any{"ok": false, "code": f.reportsErr})
			return
		}
		var req struct {
			Reports []json.RawMessage `json:"reports"`
		}
		_ = json.Unmarshal(body, &req)
		var results []map[string]any
		for _, raw := range req.Reports {
			var head struct {
				UID string `json:"report_uid"`
			}
			_ = json.Unmarshal(raw, &head)
			f.rawReports = append(f.rawReports, raw)
			switch {
			case head.UID == f.rejectUID && f.rejectUID != "":
				results = append(results, map[string]any{"report_uid": head.UID, "result": "rejected", "code": "invalid_report"})
			case f.stored[head.UID]:
				results = append(results, map[string]any{"report_uid": head.UID, "result": "duplicate"})
			default:
				f.stored[head.UID] = true
				f.order = append(f.order, head.UID)
				results = append(results, map[string]any{"report_uid": head.UID, "result": "stored"})
			}
		}
		if f.dropAnswer {
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "code": "ok", "results": results})
	default:
		http.NotFound(w, r)
	}
}

// ---- entorno de pruebas ---------------------------------------------------------------------

type env struct {
	t       *testing.T
	f       *fakeServer
	dir     string
	reports string
	store   *Store
	s       *Sender
	now     time.Time
	url     string
	paths   bool
	setURL  []string
}

func protect(p string) (string, error) {
	return "fake:" + base64.StdEncoding.EncodeToString([]byte(p)), nil
}
func open(e string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(e, "fake:"))
	return string(raw), err
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{t: t, f: newFakeServer(t), dir: t.TempDir(), now: time.Date(2026, 11, 15, 22, 0, 0, 0, time.UTC)}
	e.url = e.f.ts.URL
	e.reports = filepath.Join(e.dir, "reports")
	if err := os.MkdirAll(e.reports, 0o755); err != nil {
		t.Fatal(err)
	}
	st, err := OpenStore(filepath.Join(e.dir, "cloud.json"))
	if err != nil {
		t.Fatal(err)
	}
	e.store = st
	e.s = NewSender(Options{
		Store: st, DataDir: e.dir, ReportsDir: e.reports,
		ServerURL: func() string { return e.url }, SendPaths: func() bool { return e.paths },
		SetServerURL: func(u string) error { e.url = u; e.setURL = append(e.setURL, u); return nil },
		Describe: func() Info {
			return Info{Hostname: "PC-OFICINA", OS: "Windows 11", AgentVersion: "0.1.0", ResticVersion: "0.19.1"}
		},
		Heartbeat: func(pending int) Heartbeat {
			return Heartbeat{State: "healthy", ClientTime: time.Now().UTC().Format(time.RFC3339), QueuePending: pending}
		},
		Protect: protect, Open: open,
		MachineUID: func() (string, error) { return strings.Repeat("ab", 32), nil },
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:        func() time.Time { return e.now },
	})
	return e
}

func (e *env) report(name, uid, status string) {
	e.t.Helper()
	rep := map[string]any{"schema_version": 1, "type": "backup", "status": status, "started_at": "2026-11-15T20:00:00Z",
		"sources": []string{`C:\Clientes\García`}, "repository": `E:\Copias`, "message": "Copia correcta"}
	if uid != "" {
		rep["report_uid"] = uid
	}
	raw, _ := json.Marshal(rep)
	if err := os.WriteFile(filepath.Join(e.reports, name), raw, 0o644); err != nil {
		e.t.Fatal(err)
	}
	if err := e.store.Enqueue(name); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) enroll() {
	e.t.Helper()
	if err := e.s.Enroll(context.Background(), "BKA-AAAA-BBBB-CCCC-DDDD"); err != nil {
		e.t.Fatalf("alta: %v", err)
	}
}

func (e *env) step(force bool) { e.s.Step(context.Background(), force, false) }

func (e *env) advance(d time.Duration) { e.now = e.now.Add(d) }

// ---- pruebas ------------------------------------------------------------------------------

func TestEnrollWithLicenseKeyStoresTheKeyEncryptedAndNeverInThePlainFile(t *testing.T) {
	e := newEnv(t)
	e.enroll()
	if len(e.f.enrolls) != 1 || e.f.enrolls[0]["license_key"] != "BKA-AAAA-BBBB-CCCC-DDDD" || e.f.enrolls[0]["install_code"] != nil {
		t.Fatalf("petición: %v", e.f.enrolls)
	}
	req := e.f.enrolls[0]
	if req["machine_uid"] != strings.Repeat("ab", 32) || req["hostname"] != "PC-OFICINA" || req["agent_version"] != "0.1.0" {
		t.Fatalf("faltan datos en el alta: %v", req)
	}
	raw, _ := os.ReadFile(filepath.Join(e.dir, "cloud.json"))
	if strings.Contains(string(raw), "bkm_clave-secreta") || !strings.Contains(string(raw), "machine_key_encrypted") {
		t.Fatalf("la clave del equipo no debe estar en claro en cloud.json:\n%s", raw)
	}
	v := e.s.View()
	if v.State != StateConnected || !v.Enrolled || v.Company != "Empresa X" || v.License.MaxMachines != 2 {
		t.Fatalf("vista: %+v", v)
	}
}

func TestEnrollWithInstallCodeSendsOnlyTheCode(t *testing.T) {
	e := newEnv(t)
	if err := e.s.Enroll(context.Background(), "bki-abcde-fghjk-mnpqr-stuvw"); err != nil {
		t.Fatal(err)
	}
	if e.f.enrolls[0]["install_code"] == nil || e.f.enrolls[0]["license_key"] != nil {
		t.Fatalf("debía ir solo install_code: %v", e.f.enrolls[0])
	}
}

func TestEnrollBusinessErrorsAreReportedAndNothingIsStored(t *testing.T) {
	for _, code := range []string{"invalid_license", "invalid_install_code", "install_code_revoked", "install_code_expired", "license_expired", "license_blocked", "seat_limit_reached", "machine_revoked"} {
		t.Run(code, func(t *testing.T) {
			e := newEnv(t)
			e.f.enrollErr = code
			err := e.s.Enroll(context.Background(), "BKA-AAAA-BBBB-CCCC-DDDD")
			api, ok := err.(*APIError)
			if !ok || api.Code != code || !api.IsBusiness() || api.IsRetryable() {
				t.Fatalf("error: %#v", err)
			}
			if v := e.s.View(); v.Enrolled || v.State != StateNotEnrolled || v.Code != code {
				t.Fatalf("vista: %+v", v)
			}
		})
	}
}

func TestHeartbeatThenReportsInOrderWithoutDuplicates(t *testing.T) {
	e := newEnv(t)
	e.enroll()
	e.report("2026-11-15_20-00-00.000.json", "uid-0001-aaaa", "success")
	e.report("2026-11-15_21-00-00.000.json", "uid-0002-bbbb", "failed")
	e.report("2026-11-15_22-00-00.000.json", "uid-0003-cccc", "success")
	e.step(true)
	if e.f.hbCount != 1 || e.f.lastHB["queue_pending"].(float64) != 3 {
		t.Fatalf("latido: %d %v", e.f.hbCount, e.f.lastHB)
	}
	if got := strings.Join(e.f.order, ","); got != "uid-0001-aaaa,uid-0002-bbbb,uid-0003-cccc" {
		t.Fatalf("orden: %s", got)
	}
	if e.store.Pending() != 0 {
		t.Fatal("la cola debía vaciarse")
	}
	// Enviar otra vez no duplica ni encola nada nuevo.
	e.step(true)
	if len(e.f.order) != 3 {
		t.Fatalf("duplicados: %v", e.f.order)
	}
}

// Si la respuesta se pierde, el reenvío es «duplicate» y la cola se vacía igual.
func TestALostAnswerIsResentAndTheServerKeepsASingleCopy(t *testing.T) {
	e := newEnv(t)
	e.enroll()
	e.report("2026-11-15_20-00-00.000.json", "uid-0001-aaaa", "success")
	e.f.dropAnswer = true
	e.step(true)
	if e.store.Pending() != 1 {
		t.Fatal("sin respuesta, el informe debía seguir en la cola")
	}
	e.f.dropAnswer = false
	e.advance(2 * time.Hour)
	e.step(false)
	if e.store.Pending() != 0 || len(e.f.order) != 1 {
		t.Fatalf("pendientes=%d guardados=%v", e.store.Pending(), e.f.order)
	}
}

func TestNetworkFailureKeepsTheQueueBacksOffAndRecovers(t *testing.T) {
	e := newEnv(t)
	e.enroll()
	e.report("a.json", "uid-0001-aaaa", "success")
	e.f.down = true
	var last time.Duration
	for i := 1; i <= 4; i++ {
		e.step(true)
		d := e.store.Snapshot()
		next, _ := time.Parse(time.RFC3339, d.NextAttempt)
		wait := next.Sub(e.now)
		if d.State != StateOffline || d.Failures != i || e.store.Pending() != 1 {
			t.Fatalf("vuelta %d: %+v", i, d)
		}
		if wait < last { // la espera crece (con algo de azar, nunca menos que la anterior base)
			t.Fatalf("la espera debía crecer: %v después de %v", wait, last)
		}
		last = wait
	}
	if last < 8*time.Minute {
		t.Fatalf("tras 4 fallos la espera debía ser de unos 8 minutos: %v", last)
	}
	// Mientras toca esperar, no se llama.
	before := e.f.hbCount
	e.step(false)
	if e.f.hbCount != before {
		t.Fatal("no debía intentarlo antes de tiempo")
	}
	e.f.down = false
	e.advance(2 * time.Hour)
	e.step(false)
	if e.s.View().State != StateConnected || e.store.Pending() != 0 || len(e.f.order) != 1 {
		t.Fatalf("no se recuperó: %+v", e.s.View())
	}
}

func TestBackoffIsCappedAtOneHour(t *testing.T) {
	if d := backoff(20); d < time.Hour || d > time.Hour+7*time.Minute {
		t.Fatalf("tope: %v", d)
	}
	if d := backoff(1); d < time.Minute || d > 2*time.Minute {
		t.Fatalf("primer fallo: %v", d)
	}
}

// Licencia caducada, límite o revocado: se muestra, no se envía nada y se vuelve a mirar en horas.
func TestBusinessStatesStopSendingAndCheckAgainInHours(t *testing.T) {
	cases := map[string]string{
		"license_expired": StateLicenseExpired, "license_blocked": StateLicenseBlocked,
		"seat_limit_reached": StateSeatLimit, "machine_revoked": StateRevoked, "invalid_key": StateNeedsEnroll,
	}
	for code, state := range cases {
		t.Run(code, func(t *testing.T) {
			e := newEnv(t)
			e.enroll()
			e.report("a.json", "uid-0001-aaaa", "success")
			e.f.hbErr = code
			e.step(true)
			d := e.store.Snapshot()
			next, _ := time.Parse(time.RFC3339, d.NextAttempt)
			if e.s.View().State != state || d.Failures != 0 || next.Sub(e.now) < 5*time.Hour {
				t.Fatalf("estado %s, espera %v, %+v", e.s.View().State, next.Sub(e.now), d)
			}
			if e.store.Pending() != 1 || len(e.f.order) != 0 {
				t.Fatal("no debe enviar informes en este estado, pero tampoco perderlos")
			}
			// Una copia nueva no despierta un estado de negocio.
			before := e.f.hbCount
			e.s.Step(context.Background(), false, true)
			e.report("b.json", "uid-0002-bbbb", "success")
			e.s.Step(context.Background(), false, true)
			if e.f.hbCount != before {
				t.Fatal("no debía reintentar antes de las horas indicadas")
			}
			// Al volver la licencia, se envía todo lo acumulado.
			e.f.hbErr = ""
			e.advance(7 * time.Hour)
			e.step(false)
			if e.s.View().State != StateConnected || e.store.Pending() != 0 || len(e.f.order) != 2 {
				t.Fatalf("no se recuperó: %+v", e.s.View())
			}
		})
	}
}

func TestInvalidKeyDoesNotReEnrollByItself(t *testing.T) {
	e := newEnv(t)
	e.enroll()
	e.f.hbErr = "invalid_key"
	e.step(true)
	e.advance(7 * time.Hour)
	e.step(false)
	if len(e.f.enrolls) != 1 {
		t.Fatalf("no debe volver a darse de alta solo: %d altas", len(e.f.enrolls))
	}
	if e.s.View().State != StateNeedsEnroll {
		t.Fatalf("estado: %s", e.s.View().State)
	}
}

func TestRejectedReportsAreDroppedAndRecordedNotRetriedForever(t *testing.T) {
	e := newEnv(t)
	e.enroll()
	e.report("a.json", "uid-0001-aaaa", "success")
	e.report("b.json", "uid-0002-bbbb", "success")
	e.f.rejectUID = "uid-0001-aaaa"
	e.step(true)
	d := e.store.Snapshot()
	if len(d.Queue) != 0 || len(d.Rejected) != 1 || !strings.Contains(d.Rejected[0], "invalid_report") {
		t.Fatalf("%+v", d)
	}
}

func TestMissingOrBrokenReportFilesAreDroppedFromTheQueue(t *testing.T) {
	e := newEnv(t)
	e.enroll()
	_ = e.store.Enqueue("no-existe.json")
	_ = os.WriteFile(filepath.Join(e.reports, "roto.json"), []byte("{no es json"), 0o644)
	_ = e.store.Enqueue("roto.json")
	e.report("bien.json", "uid-0003-cccc", "success")
	e.step(true)
	if e.store.Pending() != 0 || len(e.f.order) != 1 {
		t.Fatalf("pendientes=%d guardados=%v", e.store.Pending(), e.f.order)
	}
}

// Los informes anteriores a esta versión (sin report_uid) reciben uno estable derivado del fichero.
func TestLegacyReportsGetAStableUID(t *testing.T) {
	e := newEnv(t)
	e.enroll()
	e.report("2026-11-13_13-30-00.000.json", "", "success")
	e.f.down = false
	e.step(true)
	first := append([]string(nil), e.f.order...)
	// Mismo informe otra vez (por ejemplo, la cola se reconstruyó): mismo uid, una sola ejecución.
	_ = e.store.Enqueue("2026-11-13_13-30-00.000.json")
	e.advance(time.Hour)
	e.step(true)
	if len(first) != 1 || len(e.f.order) != 1 || !strings.HasPrefix(first[0], "L") {
		t.Fatalf("uid: %v %v", first, e.f.order)
	}
}

func TestReportsLeaveWithoutFullPathsByDefault(t *testing.T) {
	e := newEnv(t)
	e.enroll()
	rep := map[string]any{"schema_version": 1, "type": "backup", "status": "warning", "started_at": "2026-11-15T20:00:00Z",
		"report_uid": "uid-0009-zzzz", "sources": []string{`C:\Clientes\García`}, "repository": `\\nas\copias-garcia`,
		"message":       `Hay ficheros que no se pudieron leer en C:\Clientes\García\Contratos\pedro.docx`,
		"errors":        []string{`No se pudo leer C:\Clientes\García\Contratos\pedro.docx: acceso denegado`},
		"skipped_files": []map[string]string{{"path": `C:\Clientes\García\Contratos\pedro.docx`, "reason": "bloqueado por otro programa"}}}
	raw, _ := json.Marshal(rep)
	_ = os.WriteFile(filepath.Join(e.reports, "x.json"), raw, 0o644)
	_ = e.store.Enqueue("x.json")
	e.step(true)
	if len(e.f.rawReports) != 1 {
		t.Fatal("no se envió")
	}
	sent := string(e.f.rawReports[0])
	for _, leak := range []string{"García", "Clientes", "pedro", "nas", "copias-garcia", `C:\`, `\\\\`} {
		if strings.Contains(sent, leak) {
			t.Fatalf("el informe enviado contiene %q:\n%s", leak, sent)
		}
	}
	if !strings.Contains(sent, ".docx") || !strings.Contains(sent, "acceso denegado") || !strings.Contains(sent, "[ruta]") {
		t.Fatalf("faltan la extensión, el motivo o el marcador:\n%s", sent)
	}
	// Con el ajuste activado, la ruta de los ficheros omitidos viaja entera.
	e.paths = true
	_ = e.store.Enqueue("x.json")
	e.f.stored = map[string]bool{}
	e.advance(time.Hour)
	e.step(true)
	if !strings.Contains(string(e.f.rawReports[len(e.f.rawReports)-1]), "pedro.docx") {
		t.Fatal("con send_paths debía ir la ruta")
	}
}

func TestHeartbeatNeverContainsTheKeyInThePayload(t *testing.T) {
	e := newEnv(t)
	e.enroll()
	e.step(true)
	b, _ := json.Marshal(e.f.lastHB)
	if strings.Contains(string(b), "bkm_") {
		t.Fatal("la clave no puede ir en el cuerpo del latido")
	}
}

// ---- alta automática con alta-backupamg.json ------------------------------------------------

func (e *env) writeAlta(v any) {
	e.t.Helper()
	raw, _ := json.Marshal(v)
	if err := os.WriteFile(filepath.Join(e.dir, AltaFileName), raw, 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) altaExists() bool {
	_, err := os.Stat(filepath.Join(e.dir, AltaFileName))
	return err == nil
}

func TestAutoEnrollFromTheInstallerFileSetsTheServerAndDeletesTheFile(t *testing.T) {
	e := newEnv(t)
	real := e.url
	e.url = "" // todavía no hay dirección: la trae el fichero
	e.writeAlta(map[string]any{"version": 1, "server_url": real, "install_code": "BKI-AAAAA-BBBBB-CCCCC-DDDDD", "company_name": "Empresa X", "campo_nuevo": 1})
	e.step(false)
	if len(e.setURL) != 1 || e.setURL[0] != real {
		t.Fatalf("no guardó la dirección: %v", e.setURL)
	}
	if e.f.enrolls[0]["install_code"] != "BKI-AAAAA-BBBBB-CCCCC-DDDDD" {
		t.Fatalf("alta: %v", e.f.enrolls)
	}
	if !e.s.View().Enrolled || e.altaExists() {
		t.Fatal("debía quedar dado de alta y sin el fichero")
	}
}

func TestAutoEnrollKeepsTheFileOnNetworkErrorAndDropsItOnDefinitiveErrors(t *testing.T) {
	e := newEnv(t)
	e.writeAlta(map[string]any{"version": 1, "server_url": e.url, "install_code": "BKI-AAAAA-BBBBB-CCCCC-DDDDD"})
	e.f.down = true
	e.step(false)
	if !e.altaExists() || e.s.View().Enrolled {
		t.Fatal("con la red caída el fichero debe conservarse")
	}
	e.f.down = false
	e.advance(2 * time.Hour)
	e.step(false)
	if !e.s.View().Enrolled || e.altaExists() {
		t.Fatalf("al volver la red debía darse de alta: %+v", e.s.View())
	}

	for _, code := range []string{"invalid_install_code", "install_code_revoked", "install_code_expired"} {
		e := newEnv(t)
		e.writeAlta(map[string]any{"version": 1, "server_url": e.url, "install_code": "BKI-AAAAA-BBBBB-CCCCC-DDDDD"})
		e.f.enrollErr = code
		e.step(false)
		if e.altaExists() || e.s.View().Enrolled || e.s.View().Code != code {
			t.Fatalf("%s: fichero=%v vista=%+v", code, e.altaExists(), e.s.View())
		}
	}
	// El límite de equipos puede cambiar: se conserva y se vuelve a mirar.
	e2 := newEnv(t)
	e2.writeAlta(map[string]any{"version": 1, "server_url": e2.url, "install_code": "BKI-AAAAA-BBBBB-CCCCC-DDDDD"})
	e2.f.enrollErr = "seat_limit_reached"
	e2.step(false)
	if !e2.altaExists() {
		t.Fatal("con el límite alcanzado el fichero se conserva")
	}
}

func TestAltaFileValidation(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, AltaFileName)
	if a, err := ReadAlta(p); a != nil || err != nil {
		t.Fatal("sin fichero no es un error")
	}
	write := func(s string) { _ = os.WriteFile(p, []byte(s), 0o644) }
	write(`{"version":2,"server_url":"https://x","install_code":"BKI-A"}`)
	if _, err := ReadAlta(p); err != ErrAltaVersion {
		t.Fatalf("una versión mayor debía rechazarse con un mensaje claro: %v", err)
	}
	write(`{"version":1,"server_url":"","install_code":"BKI-A"}`)
	if _, err := ReadAlta(p); err != ErrAltaInvalid {
		t.Fatal("sin dirección no es válido")
	}
	write(`no json`)
	if _, err := ReadAlta(p); err != ErrAltaInvalid {
		t.Fatal("json roto")
	}
	write(`{"version":1,"server_url":" https://panel.amg.desarrollo ","install_code":" BKI-A ","otra_cosa":true}`)
	a, err := ReadAlta(p)
	if err != nil || a.ServerURL != "https://panel.amg.desarrollo" || a.InstallCode != "BKI-A" {
		t.Fatalf("%+v %v", a, err)
	}
}

func TestChangingTheServerRequiresANewEnrollment(t *testing.T) {
	e := newEnv(t)
	e.enroll()
	e.url = "https://otro.servidor.example"
	if v := e.s.View(); v.State != StateNeedsEnroll {
		t.Fatalf("estado: %s", v.State)
	}
	before := e.f.hbCount
	e.step(true)
	if e.f.hbCount != before {
		t.Fatal("no debe enviar la clave a otro servidor")
	}
	if err := e.s.Forget(); err != nil {
		t.Fatal(err)
	}
	if e.s.View().Enrolled {
		t.Fatal("tras olvidar no debe seguir dado de alta")
	}
}

func TestClientRefusesInsecureAddressesAndRedirects(t *testing.T) {
	for _, bad := range []string{"", "panel.ejemplo.com", "http://panel.ejemplo.com", "ftp://x", "https://user:pw@x.com", "https://x.com/?q=1"} {
		if _, err := NewClient(bad); err == nil {
			t.Fatalf("%q debía rechazarse", bad)
		}
	}
	for _, good := range []string{"https://panel.amg.desarrollo", "https://panel.amg.desarrollo/", "https://panel.amg.desarrollo/backup/v1", "http://127.0.0.1:8080", "http://localhost"} {
		if _, err := NewClient(good); err != nil {
			t.Fatalf("%q debía valer: %v", good, err)
		}
	}
	if c, _ := NewClient("https://panel.amg.desarrollo/backup/v1/"); c.base != "https://panel.amg.desarrollo" {
		t.Fatalf("base: %s", c.base)
	}
	// Una redirección no se sigue (la clave va en la cabecera).
	var hits int
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()
	c, _ := NewClient(redirector.URL)
	_, err := c.SendHeartbeat(context.Background(), "bkm_x", Heartbeat{State: "healthy"})
	if err == nil || hits != 0 {
		t.Fatalf("no debía seguir la redirección: %v (%d)", err, hits)
	}
}

func TestNetworkErrorsNeverContainTheKey(t *testing.T) {
	c, _ := NewClient("http://127.0.0.1:1")
	_, err := c.SendHeartbeat(context.Background(), "bkm_clave-secreta", Heartbeat{State: "healthy"})
	if err == nil || strings.Contains(err.Error(), "bkm_") || strings.Contains(asAPIError(err).cause.Error(), "bkm_") {
		t.Fatalf("%v", err)
	}
	if api := asAPIError(err); !api.Network || !api.IsRetryable() {
		t.Fatalf("debía ser un error de red reintentable: %+v", api)
	}
}

func TestMachineUIDIsStableHashedAndPersonalDataFree(t *testing.T) {
	a := uidFromGUID("3F2504E0-4F89-11D3-9A0C-0305E82C3301")
	if a != uidFromGUID(" 3F2504E0-4F89-11D3-9A0C-0305E82C3301 ") || len(a) != 64 || strings.ToLower(a) != a {
		t.Fatalf("machine_uid: %s", a)
	}
	if strings.Contains(a, "3f2504e0") {
		t.Fatal("el GUID no debe verse en el valor")
	}
	if a == uidFromGUID("otro") {
		t.Fatal("debe depender del equipo")
	}
}

func TestScrubPaths(t *testing.T) {
	cases := map[string]string{
		`No se pudo leer C:\Clientes\García\a b.docx: acceso denegado`: `No se pudo leer [ruta]: acceso denegado`,
		`error en \\nas\copias\x.txt`:                                  `error en [ruta]`,
		`Sin rutas aquí`:                                               `Sin rutas aquí`,
		`hora 12:30 y letra A:`:                                        `hora 12:30 y letra A:`,
		`abre "D:\Mis cosas\f.txt" ahora`:                              `abre "[ruta]" ahora`,
		`ficheros: e:/otro/f.txt`:                                      `ficheros: [ruta]`,
	}
	for in, want := range cases {
		if got := scrub(in, false); got != want {
			t.Errorf("scrub(%q) = %q, quería %q", in, got, want)
		}
	}
	if scrub(`C:\x`, true) != `C:\x` {
		t.Fatal("con keepPaths no se toca")
	}
}

func TestPrepareReportCapsHugeReports(t *testing.T) {
	var skipped []map[string]string
	for i := 0; i < 5000; i++ {
		skipped = append(skipped, map[string]string{"path": `C:\datos\carpeta muy larga con nombre de cliente\fichero numero ` + strings.Repeat("x", 300) + ".pst", "reason": strings.Repeat("motivo ", 100)})
	}
	raw, _ := json.Marshal(map[string]any{"type": "backup", "status": "warning", "skipped_files": skipped})
	out, err := PrepareReport(raw, "uid-grande-0001", true)
	if err != nil || len(out) > maxReportBytes {
		t.Fatalf("tamaño %d: %v", len(out), err)
	}
	var back struct {
		Skipped []any `json:"skipped_files"`
		Total   int   `json:"skipped_total"`
		UID     string
	}
	_ = json.Unmarshal(out, &back)
	if len(back.Skipped) == 0 || len(back.Skipped) > maxIssues {
		t.Fatalf("incidencias: %d", len(back.Skipped))
	}
}
