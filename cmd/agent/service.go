package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"backup-agent/internal/app"
	"backup-agent/internal/logging"
	"backup-agent/internal/paths"
	"backup-agent/internal/service"
	winsvc "github.com/kardianos/service"
)

func manageService(action string) int {
	if err := service.Control(action); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", service.Describe(err))
		return 1
	}
	switch action {
	case "install":
		fmt.Fprintf(os.Stderr, "Servicio %s instalado. Arranca solo al encender el equipo.\n", app.Name)
		fmt.Fprintf(os.Stderr, "Los datos los lee de %%ProgramData%%\\%s, no de la variable %s de esta consola.\n", app.Name, app.DataDirEnv)
		fmt.Fprintf(os.Stderr, "Inícialo con: agent start\n")
	case "uninstall":
		fmt.Fprintf(os.Stderr, "Servicio %s desinstalado.\n", app.Name)
	case "start":
		if !waitUntilRunning(10 * time.Second) {
			fmt.Fprintf(os.Stderr, "El servicio se detuvo al arrancar. Revisa el log en %s\n", paths.LogFile())
			return 1
		}
		fmt.Fprintf(os.Stderr, "Servicio %s iniciado.\n", app.Name)
	case "stop":
		fmt.Fprintf(os.Stderr, "Servicio %s detenido.\n", app.Name)
	}
	return 0
}

func waitUntilRunning(limit time.Duration) bool {
	deadline := time.Now().Add(limit)
	for {
		running, err := service.Running()
		if err == nil && running {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func runInstalledService() int {
	if err := service.Run(&windowsProgram{}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", service.Describe(err))
		return 1
	}
	return 0
}

// windowsProgram atiende el arranque y la parada que pide Windows.
type windowsProgram struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func (p *windowsProgram) Start(winsvc.Service) error {
	if err := paths.Ensure(); err != nil {
		return err
	}
	logger, err := logging.New(logging.Options{
		LogFile: paths.LogFile(),
		Console: false,
	})
	if err != nil {
		return err
	}
	logger.Info("Agente iniciado",
		"version", app.Version,
		"mode", "service",
		"data_dir", paths.DataDir(),
	)
	tidyData(logger)

	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan struct{})
	go func() {
		defer close(p.done)
		defer func() { _ = logger.Close() }()
		runLoop(ctx, logger)
		logger.Info("Apagado normal")
	}()
	return nil
}

func (p *windowsProgram) Stop(winsvc.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.done == nil {
		return nil
	}
	select {
	case <-p.done:
		return nil
	case <-time.After(20 * time.Second):
		return fmt.Errorf("la parada superó el tiempo de espera")
	}
}
