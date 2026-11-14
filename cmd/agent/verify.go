package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"backup-agent/internal/app"
	"backup-agent/internal/config"
	"backup-agent/internal/logging"
	"backup-agent/internal/paths"
	"backup-agent/internal/report"
	"backup-agent/internal/restic"
	"backup-agent/internal/state"
	"backup-agent/internal/status"
	"backup-agent/internal/verify"
	"log/slog"
)

func verifyNow() int {
	cfg, password, exe, err := openRuntime()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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
	if err := executeVerification(ctx, logger.Logger, cfg, password, exe, report.TriggerManual); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func executeVerification(ctx context.Context, logger *slog.Logger, cfg *config.Config, password, exe, trigger string) error {
	ctx, stop, ok := live.enter(ctx, status.ActivityVerify)
	if !ok {
		return errBusy
	}
	defer stop()

	started := time.Now()
	logger.Info("Verificación iniciada", "subset", cfg.Verify.ReadDataSubset, "repository", cfg.Repository.Path)
	version, versionErr := restic.Version(ctx, exe)
	if versionErr != nil {
		logger.Warn("No se pudo consultar la versión de restic", "error", versionErr.Error())
	}
	result, runErr := restic.Check(ctx, exe, cfg.Repository.Path, password, cfg.Verify.ReadDataSubset)
	finished := time.Now()

	in := report.Input{
		Type:         report.TypeVerify,
		AgentVersion: app.Version,
		Hostname:     cfg.Agent.Name,
		Started:      started,
		Finished:     finished,
		Trigger:      trigger,
		Attempt:      1,
		Sources:      cfg.Backup.Sources,
		Repository:   cfg.Repository.Path,
	}
	if runErr != nil {
		in.Status = restic.StatusFailed
		in.ResticVersion = version
		in.Message = runErr.Error()
		in.Errors = []string{runErr.Error()}
	} else {
		in.Status = result.Status
		in.ResticVersion = version
		in.Message = result.Message
		if result.Status != restic.StatusSuccess {
			raw := strings.TrimSpace(result.Raw)
			if raw == "" {
				raw = result.Message
			}
			in.Errors = []string{raw}
		}
	}
	rep := report.Build(in)
	reportPath, err := report.Write(paths.ReportsDir(), rep, started)
	if err != nil {
		return err
	}
	logger.Info("Informe guardado", "path", reportPath, "type", rep.Type, "status", rep.Status)
	fmt.Fprintf(os.Stderr, "Informe: %s\n%s\n", reportPath, rep.Message)
	if err := rememberVerify(rep, filepath.Base(reportPath)); err != nil {
		logger.Error("No se pudo actualizar el estado de la verificación", "error", err.Error())
	}
	if runErr != nil {
		return runErr
	}
	if result.Status != restic.StatusSuccess {
		return fmt.Errorf("%s", result.Message)
	}
	return nil
}

func verificationDue(cfg *config.Config, now time.Time) (bool, error) {
	st, err := state.Load(paths.StatePath())
	if err != nil {
		return false, err
	}
	var last time.Time
	if st.LastVerify != nil && st.LastVerify.FinishedAt != "" {
		parsed, err := time.Parse(time.RFC3339, st.LastVerify.FinishedAt)
		if err != nil {
			return true, nil
		}
		last = parsed
	}
	return verify.Due(last, cfg.Verify.EveryDays, now), nil
}

func rememberVerify(rep report.Report, fileName string) error {
	st, err := state.Load(paths.StatePath())
	if err != nil {
		return err
	}
	st.LastVerify = &state.LastVerify{
		FinishedAt: rep.FinishedAt,
		Status:     rep.Status,
		Report:     fileName,
	}
	return state.Save(paths.StatePath(), st)
}
