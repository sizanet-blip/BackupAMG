package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"backup-agent/internal/app"
	"backup-agent/internal/config"
	"backup-agent/internal/logging"
	"backup-agent/internal/paths"
	"backup-agent/internal/report"
	"backup-agent/internal/restic"
	"backup-agent/internal/scheduler"
	"backup-agent/internal/state"
	"backup-agent/internal/status"
	"backup-agent/internal/system"
)

// Las copias adicionales (réplicas) nunca pueden estorbar a la copia principal:
//   - corren en su propio hilo, no en el del planificador;
//   - usan la misma «sala» de actividad que la copia, la verificación y la restauración;
//   - si llega la copia principal, la réplica se cancela y se reintenta después.
const (
	// replicaPoll es cada cuánto se mira si alguna copia adicional tiene algo pendiente.
	replicaPoll = 5 * time.Minute
	// replicaFailedBackoff es el mínimo entre dos intentos de una réplica que ha fallado.
	replicaFailedBackoff = 30 * time.Minute
	// replicaGuard: no se empieza una réplica si la próxima copia principal está más cerca.
	replicaGuard = 10 * time.Minute
	// replicaGuardLong es lo mismo tras replicaInterruptLimit interrupciones seguidas: así la
	// réplica espera a un hueco largo en vez de empezar y ser cortada una y otra vez.
	replicaGuardLong      = 60 * time.Minute
	replicaInterruptLimit = 3
)

var (
	// replicaMu impide dos tandas de réplicas a la vez.
	replicaMu sync.Mutex
	// replicaKick pide al hilo de réplicas que mire ya si hay algo pendiente.
	replicaKick = make(chan struct{}, 1)
	// postBackupBusy está activo mientras el planificador hace lo posterior a la copia
	// (la verificación): las réplicas esperan para no quitarle su turno.
	postBackupBusy atomic.Bool
	// stateMu serializa las escrituras de state.json de copia, verificación y réplicas.
	stateMu sync.Mutex
	// replicaRunning es la copia adicional que se está actualizando ahora, si hay.
	replicaRunning atomic.Pointer[string]
)

// replicaViews es el estado de las copias adicionales para el panel y para el semáforo.
func replicaViews(cfg *config.Config, st state.State, now time.Time) ([]status.Replica, bool) {
	if len(cfg.Replicas) == 0 {
		return nil, false
	}
	running := ""
	if id := replicaRunning.Load(); id != nil {
		running = *id
	}
	out := make([]status.Replica, 0, len(cfg.Replicas))
	anyStale := false
	for _, r := range cfg.Replicas {
		facts := status.ReplicaFacts{
			ID: r.ID, Name: r.Name, Enabled: r.Enabled, Pending: st.ReplicaPending(r.ID),
			Running: running == r.ID, Now: now,
			StaleAfter: time.Duration(cfg.Schedule.StaleAfterDays) * 24 * time.Hour,
		}
		if rs := st.Replicas[r.ID]; rs != nil {
			facts.LastStatus, facts.LastError = rs.LastStatus, rs.LastError
			facts.LastSuccessAt, facts.AddedAt = rs.LastSuccessAt, rs.AddedAt
		}
		view := status.DescribeReplica(facts)
		anyStale = anyStale || (r.Enabled && view.Stale)
		out = append(out, view)
	}
	return out, anyStale
}

func kickReplicas() {
	select {
	case replicaKick <- struct{}{}:
	default:
	}
}

// replicaLoop mira periódicamente si alguna copia adicional tiene algo pendiente.
// Cubre: tras una copia, un disco que se vuelve a conectar, el reintento tras un fallo y las
// réplicas diarias.
func replicaLoop(ctx context.Context, logger *logging.Logger) {
	timer := time.NewTimer(time.Minute) // al arrancar, primero deja que haga su copia pendiente
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		case <-replicaKick:
		}
		replicaTick(ctx, logger)
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(replicaPoll)
	}
}

func replicaTick(ctx context.Context, logger *logging.Logger) {
	if ctx.Err() != nil || postBackupBusy.Load() {
		return
	}
	retryKeyCleanup(ctx, logger)
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil || len(cfg.Replicas) == 0 {
		return
	}
	st, err := state.Load(paths.StatePath())
	if err != nil {
		logger.Error("No se pudo leer el estado de las copias adicionales", "error", err.Error())
		return
	}
	now := time.Now()
	untilNext := time.Duration(1<<62 - 1)
	if next, nextErr := scheduler.NextAfterOn(now, cfg.Schedule.Times, cfg.Schedule.Weekdays); nextErr == nil {
		untilNext = next.Sub(now)
	}
	var due []string
	for _, r := range cfg.Replicas {
		if replicaDue(r, st, now, untilNext) {
			due = append(due, r.ID)
		}
	}
	if len(due) == 0 {
		return
	}
	runReplicas(ctx, logger, due, report.TriggerScheduled)
}

// replicaDue decide si a una copia adicional le toca replicar ahora.
func replicaDue(r config.Replica, st state.State, now time.Time, untilNext time.Duration) bool {
	if !r.Enabled || !st.ReplicaPending(r.ID) {
		return false
	}
	rs := st.Replicas[r.ID]
	if rs != nil && rs.LastStatus == state.ReplicaFailed {
		if last, err := time.Parse(time.RFC3339, rs.LastAttempt); err == nil && now.Sub(last) < replicaFailedBackoff {
			return false
		}
	}
	if r.When == config.ReplicaDailyAt {
		hour, minute := 0, 0
		_, _ = fmt.Sscanf(r.Time, "%d:%d", &hour, &minute)
		slot := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if now.Before(slot) {
			return false
		}
		if rs != nil && (rs.LastStatus == state.ReplicaOK || rs.LastStatus == state.ReplicaFailed) {
			if last, err := time.Parse(time.RFC3339, rs.LastAttempt); err == nil && !last.Before(slot) {
				return false // ya se intentó en el turno de hoy
			}
		}
	}
	guard := replicaGuard
	if rs != nil && rs.Interrupted >= replicaInterruptLimit {
		guard = replicaGuardLong
	}
	return untilNext > guard
}

// runReplicas replica en las copias adicionales indicadas. Es seguro llamarlo desde cualquier
// sitio: si ya hay otra tanda o cualquier otra tarea en curso, no hace nada y se reintenta luego.
func runReplicas(ctx context.Context, logger *logging.Logger, ids []string, trigger string) {
	if !replicaMu.TryLock() {
		logger.Info("Ya hay una copia adicional en curso")
		return
	}
	defer replicaMu.Unlock()

	ctx, stop, ok := live.enter(ctx, status.ActivityReplica)
	if !ok {
		logger.Info("Hay otra tarea en curso; las copias adicionales esperan")
		return
	}
	defer stop()

	resumeSleep := system.PreventSleep()
	defer resumeSleep()

	cfg, password, exe, err := openRuntime()
	if err != nil {
		logger.Error("No se pudo preparar la copia adicional", "error", err.Error())
		return
	}
	if !resticRepo(cfg.Repository.Path) {
		logger.Info("Las copias adicionales esperan: la copia principal no está disponible o todavía no tiene copias")
		return
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		for _, r := range cfg.Replicas {
			if r.ID == id && r.Enabled {
				replicateOne(ctx, logger, cfg, password, exe, r, trigger)
			}
		}
	}
}

// replicateOne actualiza una copia adicional. Nunca toca el estado ni el informe de la copia principal.
func replicateOne(ctx context.Context, logger *logging.Logger, cfg *config.Config, password, exe string, r config.Replica, trigger string) {
	started := time.Now()
	replicaRunning.Store(&r.ID)
	defer replicaRunning.Store(nil)
	resolved, err := system.Locate(r.Path, r.VolumeID)
	if err != nil || !reachable(resolved) {
		logger.Info("La copia adicional no está conectada", "replica", r.ID, "path", r.Path)
		markReplicaOffline(logger, r.ID, started)
		return
	}
	st, _ := state.Load(paths.StatePath())
	good := st.GoodSnapshot()

	logger.Info("Copia adicional iniciada", "replica", r.ID, "name", r.Name, "path", resolved, "trigger", trigger)
	freeBytes, _ := system.FreeBytes(resolved)
	copied, runErr, cancelled := replicaCopy(ctx, logger, cfg, password, exe, resolved)
	finished := time.Now()

	if cancelled {
		logger.Info("La copia adicional se ha interrumpido; continuará después", "replica", r.ID)
		recordReplica(logger, r.ID, func(rs *state.ReplicaState) {
			rs.LastAttempt = finished.Format(time.RFC3339)
			rs.LastStatus = state.ReplicaInterrupted
			rs.Interrupted++
		})
		return
	}

	in := report.Input{
		Type:                 report.TypeReplica,
		AgentVersion:         app.Version,
		Hostname:             cfg.Agent.Name,
		Started:              started,
		Finished:             finished,
		Trigger:              trigger,
		Attempt:              1,
		Sources:              cfg.Backup.Sources,
		Repository:           resolved,
		DestinationFreeBytes: freeBytes,
		ReplicaID:            r.ID,
		ReplicaName:          r.Name,
		SnapshotsCopied:      copied,
	}
	if version, vErr := restic.Version(ctx, exe); vErr == nil {
		in.ResticVersion = version
	}
	if runErr != nil {
		in.Status = restic.StatusFailed
		in.Message = runErr.Error()
		in.Errors = []string{runErr.Error()}
	} else {
		in.Status = restic.StatusSuccess
		in.Message = "Copia adicional al día"
		if copied > 0 {
			in.Message = "Copia adicional actualizada"
			// La retención va aparte: si falla, la réplica sigue siendo correcta.
			retention, retErr := restic.Forget(ctx, exe, resolved, password, restic.RetentionPolicy{
				KeepDaily:   cfg.Retention.KeepDaily,
				KeepWeekly:  cfg.Retention.KeepWeekly,
				KeepMonthly: cfg.Retention.KeepMonthly,
				KeepYearly:  cfg.Retention.KeepYearly,
				Prune:       cfg.Retention.Prune,
			})
			switch {
			case retErr != nil:
				logger.Error("No se pudo aplicar la retención en la copia adicional", "replica", r.ID, "error", retErr.Error())
				in.Errors = append(in.Errors, retErr.Error())
			default:
				in.RetentionApplied = retention.Applied
				in.SnapshotsRemoved = retention.SnapshotsRemoved
			}
			in.Finished = time.Now()
			finished = in.Finished
		}
	}

	reportName := ""
	if runErr != nil || copied > 0 {
		rep := report.Build(in)
		if path, wErr := report.Write(paths.ReportsDir(), rep, started); wErr != nil {
			logger.Error("No se pudo guardar el informe de la copia adicional", "error", wErr.Error())
		} else {
			reportName = filepath.Base(path)
			logger.Info("Informe guardado", "path", path, "type", rep.Type, "status", rep.Status)
			cloudReportWritten(path)
		}
	}
	recordReplica(logger, r.ID, func(rs *state.ReplicaState) {
		rs.LastAttempt = finished.Format(time.RFC3339)
		rs.Interrupted = 0
		if reportName != "" {
			rs.Report = reportName
		}
		if runErr != nil {
			rs.LastStatus = state.ReplicaFailed
			rs.LastError = restic.PlainFailure(runErr.Error())
			return
		}
		rs.LastStatus = state.ReplicaOK
		rs.LastError = ""
		rs.LastSuccessAt = finished.Format(time.RFC3339)
		rs.LastSnapshot = good
	})
	if runErr != nil {
		logger.Error("La copia adicional ha fallado", "replica", r.ID, "error", runErr.Error())
		return
	}
	logger.Info(in.Message, "replica", r.ID, "snapshots_copied", copied)
}

// replicaCopy prepara el repositorio secundario si hace falta y replica en él.
func replicaCopy(ctx context.Context, logger *logging.Logger, cfg *config.Config, password, exe, dest string) (int, error, bool) {
	if !resticRepo(dest) {
		logger.Info("Preparando la copia adicional", "path", dest)
		if _, err := restic.InitReplica(ctx, restic.ReplicaInitRequest{
			Executable: exe, Repository: dest, Source: cfg.Repository.Path, Password: password,
		}); err != nil {
			if ctx.Err() != nil {
				return 0, nil, true
			}
			return 0, err, false
		}
	}
	if host, err := os.Hostname(); err == nil {
		if n, err := restic.ReleaseStale(ctx, exe, dest, password, host, time.Now(), system.PIDAlive); n > 0 {
			logger.Warn("Eliminando bloqueos antiguos de la copia adicional", "count", n)
		} else if err != nil {
			logger.Warn("No se pudo revisar los bloqueos de la copia adicional", "error", err.Error())
		}
	}
	result, err := restic.Copy(ctx, restic.CopyRequest{
		Executable: exe, Source: cfg.Repository.Path, Destination: dest, Password: password,
	})
	if err != nil {
		return result.Copied, err, false
	}
	return result.Copied, nil, result.Cancelled
}

func markReplicaOffline(logger *logging.Logger, id string, now time.Time) {
	st, err := state.Load(paths.StatePath())
	if err == nil {
		if rs := st.Replicas[id]; rs != nil && rs.LastStatus == state.ReplicaOffline {
			return // no se reescribe el estado en cada comprobación
		}
	}
	recordReplica(logger, id, func(rs *state.ReplicaState) {
		rs.LastAttempt = now.Format(time.RFC3339)
		rs.LastStatus = state.ReplicaOffline
	})
}

// recordReplica cambia el estado de una copia adicional sin tocar el resto de state.json.
func recordReplica(logger *logging.Logger, id string, change func(*state.ReplicaState)) {
	stateMu.Lock()
	defer stateMu.Unlock()
	st, err := state.Load(paths.StatePath())
	if err != nil {
		logger.Error("No se pudo leer el estado", "error", err.Error())
		return
	}
	if st.Replicas == nil {
		st.Replicas = map[string]*state.ReplicaState{}
	}
	rs := st.Replicas[id]
	if rs == nil {
		rs = &state.ReplicaState{}
		st.Replicas[id] = rs
	}
	if rs.AddedAt == "" {
		rs.AddedAt = time.Now().Format(time.RFC3339)
	}
	change(rs)
	if err := state.Save(paths.StatePath(), st); err != nil {
		logger.Error("No se pudo actualizar el estado de la copia adicional", "error", err.Error())
	}
}

// preemptReplicas da prioridad a la copia principal: si hay una réplica en curso, la cancela y
// espera a que suelte la sala. La réplica queda pendiente y se retoma después.
func preemptReplicas(logger *logging.Logger) {
	if !live.cancelKind(status.ActivityReplica) {
		return
	}
	logger.Info("La copia principal tiene prioridad: se interrumpe la copia adicional")
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) && !live.idle() {
		time.Sleep(100 * time.Millisecond)
	}
}

// replicateNow es el comando `agent replicate [id]`: replica ya, sin esperar huecos.
func replicateNow(args []string) int {
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	var ids []string
	for _, r := range cfg.Replicas {
		if len(args) == 0 || args[0] == r.ID {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 {
		fmt.Fprintln(os.Stderr, "No hay copias adicionales que coincidan en la configuración.")
		return 1
	}
	logger, err := logging.New(logging.Options{LogFile: paths.LogFile(), Console: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	defer func() { _ = logger.Close() }()
	tidyData(logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runReplicas(ctx, logger, ids, report.TriggerManual)

	st, err := state.Load(paths.StatePath())
	if err != nil {
		return 1
	}
	code := 0
	for _, id := range ids {
		rs := st.Replicas[id]
		switch {
		case rs == nil:
			fmt.Fprintf(os.Stderr, "%s: sin intentos todavía\n", id)
			code = 1
		case rs.LastStatus == state.ReplicaOK && !st.ReplicaPending(id):
			fmt.Fprintf(os.Stderr, "%s: al día\n", id)
		default:
			fmt.Fprintf(os.Stderr, "%s: %s %s\n", id, rs.LastStatus, rs.LastError)
			code = 1
		}
	}
	return code
}
