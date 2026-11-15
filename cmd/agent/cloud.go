package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"backup-agent/internal/app"
	"backup-agent/internal/cloud"
	"backup-agent/internal/config"
	"backup-agent/internal/logging"
	"backup-agent/internal/paths"
	"backup-agent/internal/restic"
	"backup-agent/internal/secrets"
	"backup-agent/internal/state"
	"backup-agent/internal/status"
	"backup-agent/internal/system"
)

// La conexión con la plataforma web vive en su propio hilo: un fallo de red, una licencia caducada
// o un límite de equipos solo cambian lo que se muestra. Nunca afectan a las copias.

var (
	cloudOnce   sync.Once
	cloudSender *cloud.Sender
	cloudStore  *cloud.Store
	cloudErr    error
)

// initCloud prepara el envío una sola vez. Devuelve nil si no se pudo (las copias siguen igual).
func initCloud() *cloud.Sender {
	cloudOnce.Do(func() {
		var store *cloud.Store
		store, cloudErr = cloud.OpenStore(filepath.Join(paths.DataDir(), "cloud.json"))
		if cloudErr != nil {
			return
		}
		cloudStore = store
		cloudSender = cloud.NewSender(cloud.Options{
			Store:      store,
			DataDir:    paths.DataDir(),
			ReportsDir: paths.ReportsDir(),
			ServerURL:  func() string { return cloudConfig().ServerURL },
			SendPaths:  func() bool { return cloudConfig().SendPaths },
			SetServerURL: func(url string) error {
				cur := cloudConfig()
				cur.ServerURL = url
				return config.SetCloud(paths.ConfigPath(), cur)
			},
			Describe:   cloudInfo,
			Heartbeat:  buildHeartbeat,
			Protect:    secrets.Protect,
			Open:       secrets.Open,
			MachineUID: cloud.MachineUID,
			Log:        nil, // se fija en startCloud
		})
	})
	return cloudSender
}

func cloudConfig() config.Cloud {
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		return config.Cloud{}
	}
	return cfg.Cloud
}

var (
	resticVersionOnce sync.Once
	resticVersionText string
)

func cloudInfo() cloud.Info {
	info := cloud.Info{OS: system.OSVersion(), AgentVersion: app.Version}
	if cfg, err := config.Load(paths.ConfigPath()); err == nil {
		info.Hostname = cfg.Agent.Name
	}
	resticVersionOnce.Do(func() {
		exe, err := restic.Executable()
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if v, err := restic.Version(ctx, exe); err == nil {
			resticVersionText = v
		}
	})
	info.ResticVersion = resticVersionText
	return info
}

// startCloud arranca el hilo de envío. Si algo falla al prepararlo, se anota y las copias siguen igual.
func startCloud(ctx context.Context, logger *logging.Logger) {
	sender := initCloud()
	if sender == nil {
		logger.Warn("No se pudo preparar la conexión con la plataforma", "error", cloudErr.Error())
		return
	}
	sender.SetLogger(logger.Logger)
	go sender.Run(ctx)
}

// cloudReportWritten anota un informe recién escrito para enviarlo y avisa al hilo de envío.
func cloudReportWritten(reportPath string) {
	sender := initCloud()
	if sender == nil || cloudStore == nil {
		return
	}
	if err := cloudStore.Enqueue(filepath.Base(reportPath)); err != nil {
		return // no se pudo anotar: el informe sigue en su sitio y las copias no se ven afectadas
	}
	sender.Kick()
}

// cloudStatus es el resumen para el panel y la bandeja.
func cloudStatus() *status.Cloud {
	sender := initCloud()
	if sender == nil {
		return nil
	}
	v := sender.View()
	return &status.Cloud{State: v.State, Code: v.Code, Company: v.Company, Pending: v.Pending, LastSyncAt: v.LastSyncAt}
}

// buildHeartbeat resume el estado del equipo para la plataforma.
func buildHeartbeat(pending int) cloud.Heartbeat {
	now := time.Now()
	hb := cloud.Heartbeat{State: "unknown", ClientTime: now.UTC().Format(time.RFC3339), QueuePending: pending}
	info := cloudInfo()
	hb.Hostname, hb.OS, hb.AgentVersion, hb.ResticVersion = info.Hostname, info.OS, info.AgentVersion, info.ResticVersion

	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		return hb
	}
	st, _ := state.Load(paths.StatePath())
	view, _ := live.snapshot().(status.View)

	hb.State = map[string]string{status.Green: "healthy", status.Yellow: "warning", status.Red: "failed", status.Blue: "busy", "gray": "stopped"}[view.Level]
	if hb.State == "" {
		hb.State = "unknown"
	}
	if st.LastBackup != nil && st.LastBackup.FinishedAt != "" {
		hb.LastBackupAt = strp(st.LastBackup.FinishedAt)
	}
	good := st.LastGoodAt
	if good == "" && st.LastBackup != nil && st.LastBackup.Status != "failed" {
		good = st.LastBackup.FinishedAt
	}
	if good != "" {
		hb.LastSuccessAt = strp(good)
	}
	if view.NextBackup != "" {
		hb.NextBackupAt = strp(view.NextBackup)
	}
	if st.LastVerify != nil && st.LastVerify.FinishedAt != "" {
		hb.LastVerifyAt = strp(st.LastVerify.FinishedAt)
		ok := st.LastVerify.Status == "success"
		hb.LastVerifyOK = &ok
	}
	if due, err := verificationDue(cfg, now); err == nil {
		hb.VerifyDue = due
	}
	hb.DestFreeBytes, hb.DestTotalBytes = view.Destination.FreeBytes, view.Destination.TotalBytes

	primaryState := "up_to_date" // valores que entiende la plataforma: up_to_date, pending, offline, error, unknown
	switch {
	case !view.Destination.Connected:
		primaryState = "offline"
	case st.LastBackup != nil && st.LastBackup.Status == "failed":
		primaryState = "error"
	}
	primaryType := "local"
	if view.Destination.Kind == "unc" || view.Destination.Kind == "remote" {
		primaryType = "unc"
	}
	primary := cloud.Destination{ID: "principal", Name: "Copia principal", Type: primaryType, State: primaryState, LastOKAt: good}
	if view.Destination.UsedKnown {
		primary.UsedBytes = view.Destination.UsedBytes
	}
	hb.Destinations = append(hb.Destinations, primary)
	kinds := map[string]string{}
	for _, r := range cfg.Replicas {
		kinds[r.ID] = r.Type
	}
	stateNames := map[string]string{status.ReplicaOK: "up_to_date", status.ReplicaPending: "pending", status.ReplicaOffline: "offline", status.ReplicaFailed: "error", status.ReplicaDisabled: "unknown"}
	for _, r := range view.Replicas {
		if len(hb.Destinations) >= 10 {
			break
		}
		hb.Destinations = append(hb.Destinations, cloud.Destination{
			ID: r.ID, Name: r.Name, Type: kinds[r.ID], State: stateNames[r.State], LastOKAt: r.LastSuccessAt,
		})
	}
	return hb
}

func strp(s string) *string { return &s }

// enrollCommand es `agent enroll [--server URL] <clave de licencia o código de instalación>`.
func enrollCommand(args []string) int {
	server, credential := "", ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--server" && i+1 < len(args):
			server = args[i+1]
			i++
		case credential == "":
			credential = args[i]
		default:
			fmt.Fprintln(os.Stderr, "Uso: agent enroll [--server https://panel.ejemplo.com] <clave de licencia o código de instalación>")
			return 2
		}
	}
	if credential == "" {
		fmt.Fprintln(os.Stderr, "Uso: agent enroll [--server https://panel.ejemplo.com] <clave de licencia o código de instalación>")
		return 2
	}
	return runEnroll(server, credential)
}

// enrollFileCommand es `agent enroll-file [ruta]`: alta con alta-backupamg.json.
func enrollFileCommand(args []string) int {
	path := filepath.Join(paths.DataDir(), cloud.AltaFileName)
	if len(args) == 1 {
		path = args[0]
	}
	alta, err := cloud.ReadAlta(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	if alta == nil {
		fmt.Fprintf(os.Stderr, "No se encontró %s\n", path)
		return 1
	}
	code := runEnroll(alta.ServerURL, alta.InstallCode)
	if code == 0 && len(args) == 0 {
		_ = os.Remove(path)
	}
	return code
}

func runEnroll(server, credential string) int {
	if err := paths.Ensure(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	sender := initCloud()
	if sender == nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", cloudErr)
		return 1
	}
	if server != "" {
		cur := cloudConfig()
		if strings.TrimRight(cur.ServerURL, "/") != strings.TrimRight(strings.TrimSpace(server), "/") {
			cur.ServerURL = server
			if err := config.SetCloud(paths.ConfigPath(), cur); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return 1
			}
			_ = sender.Forget()
		}
	}
	if cloudConfig().ServerURL == "" {
		fmt.Fprintln(os.Stderr, "Falta la dirección de la plataforma: usa --server https://panel.ejemplo.com")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := sender.Enroll(ctx, credential); err != nil {
		fmt.Fprintf(os.Stderr, "No se pudo dar de alta el equipo: %s\n", cloudProblem(err))
		return 1
	}
	v := sender.View()
	fmt.Fprintf(os.Stderr, "Equipo dado de alta en la plataforma (%s). Equipos de la licencia: %d de %d.\n", v.Company, v.License.UsedMachines, v.License.MaxMachines)
	return 0
}
