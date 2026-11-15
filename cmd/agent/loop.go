package main

import (
	"context"
	"strings"
	"sync"

	"backup-agent/internal/config"
	"backup-agent/internal/logging"
	"backup-agent/internal/paths"
)

// setupReady se cierra cuando el asistente deja una configuración válida.
var setupReady = make(chan struct{})
var signalSetupOnce sync.Once

func signalSetup() {
	signalSetupOnce.Do(func() { close(setupReady) })
}

// runLoop arranca el panel y, si falta la configuración, espera al asistente antes de programar copias.
func runLoop(ctx context.Context, logger *logging.Logger) int {
	paths.CleanTemp()
	cfg, loadErr := config.Load(paths.ConfigPath())
	port := config.PeekPort(paths.ConfigPath())
	if loadErr == nil {
		port = cfg.Panel.Port
		logger.Info("Configuración cargada",
			"agent", cfg.Agent.Name,
			"sources", len(cfg.Backup.Sources),
			"repository", cfg.Repository.Path,
			"times", strings.Join(cfg.Schedule.Times, ", "),
			"vss", cfg.Backup.UseVSS,
		)
	} else {
		logger.Warn("No hay una configuración válida. El asistente está en el panel.", "error", loadErr.Error())
	}
	startPanel(ctx, logger, port)
	if loadErr != nil {
		select {
		case <-ctx.Done():
			return 0
		case <-setupReady:
		}
		var err error
		cfg, err = loadLoggedConfig(logger)
		if err != nil {
			return 1
		}
	}
	go replicaLoop(ctx, logger)
	runSchedule(ctx, logger, cfg)
	return 0
}
