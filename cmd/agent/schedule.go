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

func runSchedule(ctx context.Context, logger *logging.Logger, cfg *config.Config) {
	if cfg.Schedule.RunMissedOnStartup {
		if slot, ok := scheduler.MostRecentDueOn(time.Now(), lastAttemptTime(), cfg.Schedule.Times, cfg.Schedule.Weekdays); ok {
			trigger := report.TriggerMissedOnStartup
			if scheduler.SameMinute(slot, time.Now()) {
				trigger = report.TriggerScheduled
			}
			logger.Info("Hay una copia pendiente", "slot", slot.Format(time.RFC3339), "trigger", trigger)
			runWithRetries(ctx, logger, trigger)
		}
	}
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
			runWithRetries(ctx, logger, trigger)
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
