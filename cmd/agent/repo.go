package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"backup-agent/internal/app"
	"backup-agent/internal/config"
	"backup-agent/internal/logging"
	"backup-agent/internal/paths"
	"backup-agent/internal/report"
	"backup-agent/internal/restic"
	"backup-agent/internal/secrets"
	"backup-agent/internal/state"
	"backup-agent/internal/system"
)

func initRepo() int {
	cfg, password, exe, err := openRuntime()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	result, err := restic.Init(ctx, restic.InitRequest{
		Executable: exe,
		Repository: cfg.Repository.Path,
		Password:   password,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintln(os.Stderr, result.Message)
	if logger, err := logging.New(logging.Options{LogFile: paths.LogFile(), Console: false}); err == nil {
		logger.Info(result.Message, "repository", cfg.Repository.Path, "id", result.ID)
		_ = logger.Close()
	}
	return 0
}

func backupNow() int {
	logger, err := logging.New(logging.Options{LogFile: paths.LogFile(), Console: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	defer func() { _ = logger.Close() }()

	tidyData(logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	code := runWithRetries(ctx, logger, report.TriggerManual)
	if code < 0 {
		logger.Info("Apagado normal")
		return 0
	}
	return code
}

func oneAttempt(ctx context.Context, logger *logging.Logger, cfg *config.Config, password, exe, trigger string, attempt int) int {
	tidyData(logger)
	if err := releaseStaleLocks(ctx, logger, cfg, password, exe); err != nil {
		logger.Error("No se pudo revisar los bloqueos del repositorio", "error", err.Error())
	}
	started := time.Now()
	freeBytes, freeErr := system.FreeBytes(cfg.Repository.Path)
	if freeErr != nil {
		logger.Warn("No se pudo consultar el espacio libre", "error", freeErr.Error())
	}
	logger.Info("Copia iniciada",
		"repository", cfg.Repository.Path,
		"vss", cfg.Backup.UseVSS,
		"free_bytes", freeBytes,
		"trigger", trigger,
		"attempt", attempt,
	)

	lastPercent := -1
	outcome, runErr := restic.Backup(ctx, restic.BackupRequest{
		Executable: exe,
		Repository: cfg.Repository.Path,
		Password:   password,
		Sources:    cfg.Backup.Sources,
		Excludes:   cfg.Backup.Excludes,
		UseVSS:     cfg.Backup.UseVSS,
		Host:       cfg.Agent.Name,
		OnStatus: func(progress restic.Status) {
			live.note(progress)
			percent := int(progress.PercentDone * 100)
			if percent == lastPercent {
				return
			}
			lastPercent = percent
			logger.Info("Progreso de la copia",
				"percent", percent,
				"files_done", progress.FilesDone,
				"files_total", progress.TotalFiles,
			)
		},
	})
	finished := time.Now()
	var retention restic.RetentionResult
	var retentionErr error
	if runErr == nil && outcome.SnapshotID != "" {
		logger.Info("Aplicando retención",
			"daily", cfg.Retention.KeepDaily,
			"weekly", cfg.Retention.KeepWeekly,
			"monthly", cfg.Retention.KeepMonthly,
			"yearly", cfg.Retention.KeepYearly,
			"prune", cfg.Retention.Prune,
		)
		retention, retentionErr = restic.Forget(ctx, exe, cfg.Repository.Path, password, restic.RetentionPolicy{
			KeepDaily:   cfg.Retention.KeepDaily,
			KeepWeekly:  cfg.Retention.KeepWeekly,
			KeepMonthly: cfg.Retention.KeepMonthly,
			KeepYearly:  cfg.Retention.KeepYearly,
			Prune:       cfg.Retention.Prune,
		})
		if retentionErr != nil {
			logger.Error("No se pudo aplicar la retención", "error", retentionErr.Error())
		} else {
			logger.Info(retention.Message, "removed", retention.SnapshotsRemoved)
		}
		finished = time.Now()
	}
	rep := backupReport(cfg, started, finished, freeBytes, outcome, runErr, retention, retentionErr, trigger, attempt)
	reportPath, reportErr := report.Write(paths.ReportsDir(), rep, started)
	if reportErr != nil {
		logger.Error("No se pudo guardar el informe", "error", reportErr.Error())
		fmt.Fprintf(os.Stderr, "Error: %v\n", reportErr)
	} else {
		logger.Info("Informe guardado", "path", reportPath, "status", rep.Status)
		fmt.Fprintf(os.Stderr, "Informe: %s\n", reportPath)
		if err := rememberBackup(rep, filepath.Base(reportPath)); err != nil {
			logger.Error("No se pudo actualizar el estado", "error", err.Error())
		}
	}

	if runErr != nil {
		logger.Error("La copia no se ha realizado", "error", runErr.Error())
		fmt.Fprintf(os.Stderr, "Error: %v\n", runErr)
		if ctx.Err() != nil {
			return -1
		}
		return 1
	}
	logger.Info(outcome.Message,
		"status", outcome.Status,
		"exit_code", outcome.ExitCode,
		"snapshot", outcome.SnapshotID,
		"files_new", outcome.FilesNew,
		"files_changed", outcome.FilesChanged,
		"skipped", len(outcome.Skipped),
		"restic", outcome.ResticVersion,
	)
	for _, file := range outcome.Skipped {
		logger.Warn("Fichero no leído", "path", file.Path, "reason", file.Reason)
	}
	fmt.Fprintln(os.Stderr, outcome.Text())
	if outcome.Message == "La copia se ha cancelado" || ctx.Err() != nil {
		return -1
	}
	if reportErr != nil || outcome.Status == restic.StatusFailed {
		return 1
	}
	if outcome.Status == restic.StatusWarning {
		return 3
	}
	return 0
}

func backupReport(cfg *config.Config, started, finished time.Time, freeBytes int64, outcome restic.BackupOutcome, runErr error, retention restic.RetentionResult, retentionErr error, trigger string, attempt int) report.Report {
	in := report.Input{
		AgentVersion:         app.Version,
		Hostname:             cfg.Agent.Name,
		Started:              started,
		Finished:             finished,
		Trigger:              trigger,
		Attempt:              attempt,
		Sources:              cfg.Backup.Sources,
		Repository:           cfg.Repository.Path,
		DestinationFreeBytes: freeBytes,
	}
	if runErr != nil {
		in.Status = restic.StatusFailed
		in.Message = runErr.Error()
		in.Errors = []string{runErr.Error()}
		return report.Build(in)
	}
	in.ResticVersion = outcome.ResticVersion
	in.Status = outcome.Status
	in.VSSUsed = cfg.Backup.UseVSS
	in.FilesNew = outcome.FilesNew
	in.FilesChanged = outcome.FilesChanged
	in.FilesUnmodified = outcome.FilesUnmodified
	in.DataAddedBytes = outcome.DataAdded
	in.TotalFilesProcessed = outcome.TotalFilesProcessed
	in.TotalBytesProcessed = outcome.TotalBytesProcessed
	in.SnapshotID = outcome.SnapshotID
	in.Message = outcome.Message
	in.RetentionApplied = retention.Applied
	in.SnapshotsRemoved = retention.SnapshotsRemoved
	for _, file := range outcome.Skipped {
		in.SkippedFiles = append(in.SkippedFiles, report.SkippedFile{Path: file.Path, Reason: file.Reason})
	}
	if outcome.Status == restic.StatusFailed && outcome.Message != "" {
		in.Errors = append(in.Errors, outcome.Message)
	}
	if retentionErr != nil {
		in.Errors = append(in.Errors, retentionErr.Error())
	} else if retention.ExitCode == 3 && retention.Message != "" {
		in.Errors = append(in.Errors, retention.Message)
	}
	return report.Build(in)
}

func rememberBackup(rep report.Report, fileName string) error {
	st, err := state.Load(paths.StatePath())
	if err != nil {
		return err
	}
	st.LastBackup = &state.LastBackup{
		FinishedAt: rep.FinishedAt,
		Status:     rep.Status,
		SnapshotID: rep.SnapshotID,
		Report:     fileName,
	}
	return state.Save(paths.StatePath(), st)
}

func openRuntime() (*config.Config, string, string, error) {
	if err := paths.Ensure(); err != nil {
		return nil, "", "", err
	}
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		return nil, "", "", err
	}
	password, err := secrets.Open(cfg.Repository.PasswordEncrypted)
	if err != nil {
		return nil, "", "", err
	}
	resolved, err := system.Locate(cfg.Repository.Path, cfg.Repository.VolumeID)
	if err != nil {
		return nil, "", "", err
	}
	cfg.Repository.Path = resolved
	exe, err := restic.Executable()
	if err != nil {
		return nil, "", "", err
	}
	return cfg, password, exe, nil
}
