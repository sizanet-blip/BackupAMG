package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

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

var backupMu sync.Mutex

// reloadConfig despierta el planificador cuando el panel guarda la configuración.
var reloadConfig = make(chan struct{}, 1)

func signalConfigReload() {
	select {
	case reloadConfig <- struct{}{}:
	default:
	}
	kickReplicas()
}

// retryWait es el tiempo de espera entre reintentos. Las pruebas lo acortan.
var retryWait = func(minutes int) time.Duration {
	return time.Duration(minutes) * time.Minute
}

func runWithRetries(ctx context.Context, logger *logging.Logger, trigger string) int {
	if !backupMu.TryLock() {
		logger.Warn("Ya hay una copia en curso; esta no se inicia")
		return 1
	}
	defer backupMu.Unlock()

	waitPasswordChange(ctx)
	preemptReplicas(logger)
	ctx, stop, ok := live.enter(ctx, status.ActivityBackup)
	if !ok {
		logger.Warn("Ya hay una tarea en curso; esta no se inicia")
		return 1
	}
	defer stop()

	resumeSleep := system.PreventSleep()
	defer resumeSleep()

	cfg, password, exe, err := openRuntime()
	if err != nil {
		logger.Error("No se pudo preparar la copia", "error", err.Error())
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	attempt := 1
	current := trigger
	live.setCue(current, cfg.Backup.UseVSS)
	for {
		if ctx.Err() != nil {
			return -1
		}
		code := oneAttempt(ctx, logger, cfg, password, exe, current, attempt)
		if code == 0 || code == 3 || code < 0 {
			return code
		}
		if !scheduler.ShouldRetry(attempt, cfg.Schedule.MaxRetries) {
			return code
		}
		delay := retryWait(cfg.Schedule.RetryDelayMinutes)
		logger.Info("La copia ha fallado; se reintentará",
			"attempt", attempt+1,
			"wait_minutes", cfg.Schedule.RetryDelayMinutes,
		)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return -1
		case <-timer.C:
		}
		attempt++
		current = report.TriggerRetry
		live.setCue(current, cfg.Backup.UseVSS)
	}
}

// afterBackup hace lo que toca después de una copia: la verificación periódica, si toca.
// Solo tras una copia correcta o con avisos. Si la verificación se alarga y se pasa la hora de la
// siguiente copia, esa copia se hace nada más acabar, para que no se pierda.
func afterBackup(ctx context.Context, logger *logging.Logger, cfg *config.Config, code int) {
	// Las copias adicionales esperan a que acabe la verificación y luego se ponen al día.
	postBackupBusy.Store(true)
	defer func() {
		postBackupBusy.Store(false)
		kickReplicas()
	}()
	if code != 0 && code != 3 {
		return
	}
	next, nextErr := scheduler.NextAfterOn(time.Now(), cfg.Schedule.Times, cfg.Schedule.Weekdays)
	if !verifyIfDue(ctx, logger) || nextErr != nil || ctx.Err() != nil {
		return
	}
	if !time.Now().Before(next) {
		logger.Warn("La verificación ha durado más que el hueco hasta la próxima copia; se hace ahora", "slot", next.Format(time.RFC3339))
		runWithRetries(ctx, logger, report.TriggerMissedOnStartup)
	}
}

func runSchedule(ctx context.Context, logger *logging.Logger, cfg *config.Config) {
	startupCode := 0
	if cfg.Schedule.RunMissedOnStartup {
		if slot, ok := scheduler.MostRecentDueOn(time.Now(), lastAttemptTime(), cfg.Schedule.Times, cfg.Schedule.Weekdays); ok {
			trigger := report.TriggerMissedOnStartup
			if scheduler.SameMinute(slot, time.Now()) {
				trigger = report.TriggerScheduled
			}
			logger.Info("Hay una copia pendiente", "slot", slot.Format(time.RFC3339), "trigger", trigger)
			startupCode = runWithRetries(ctx, logger, trigger)
		}
	}
	afterBackup(ctx, logger, cfg, startupCode)
	for {
		if ctx.Err() != nil {
			return
		}
		if loaded, loadErr := config.Load(paths.ConfigPath()); loadErr != nil {
			logger.Error("No se pudo releer la configuración", "error", loadErr.Error())
		} else {
			cfg = loaded
		}
		next, err := scheduler.NextAfterOn(time.Now(), cfg.Schedule.Times, cfg.Schedule.Weekdays)
		if err != nil {
			logger.Error("No se pudo calcular la próxima copia", "error", err.Error())
			return
		}
		logger.Info("Próxima copia", "at", next.Format(time.RFC3339))
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-reloadConfig:
			timer.Stop()
			logger.Info("Configuración recargada")
			continue
		case <-timer.C:
			trigger := report.TriggerScheduled
			if time.Since(next) > time.Minute && cfg.Schedule.RunMissedOnStartup {
				trigger = report.TriggerMissedOnStartup
			}
			code := runWithRetries(ctx, logger, trigger)
			afterBackup(ctx, logger, cfg, code)
		}
	}
}

func lastAttemptTime() time.Time {
	st, err := state.Load(paths.StatePath())
	if err != nil || st.LastBackup == nil || st.LastBackup.FinishedAt == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, st.LastBackup.FinishedAt)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func releaseStaleLocks(ctx context.Context, logger *logging.Logger, cfg *config.Config, password, exe string) error {
	host, err := os.Hostname()
	if err != nil {
		return err
	}
	n, err := restic.ReleaseStale(ctx, exe, cfg.Repository.Path, password, host, time.Now(), system.PIDAlive)
	if n > 0 {
		logger.Warn("Eliminando bloqueos antiguos del repositorio", "count", n)
	}
	return err
}
