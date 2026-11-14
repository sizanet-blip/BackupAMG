package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"backup-agent/internal/housekeeping"
	"backup-agent/internal/logging"
	"backup-agent/internal/panel"
	"backup-agent/internal/paths"
	"backup-agent/internal/system"
)

func tidyData(logger *logging.Logger) {
	removed, err := housekeeping.Prune(paths.ReportsDir(), paths.LogsDir(), time.Now())
	if err != nil {
		logger.Warn("No se pudieron limpiar informes o logs antiguos", "error", err.Error())
		return
	}
	if removed > 0 {
		logger.Info("Eliminados informes o logs antiguos", "count", removed, "keep_days", housekeeping.KeepDays)
	}
}

func secureData() int {
	root := paths.ProgramDataRoot()
	for _, dir := range []string{root, filepath.Join(root, "reports"), filepath.Join(root, "logs")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
	}
	if err := system.ProtectTree(root); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "Permisos restringidos en %s: solo SYSTEM y administradores pueden leer la configuración.\n", root)
	return 0
}

func startPanel(ctx context.Context, logger *logging.Logger, port int) {
	srv := panel.New(panel.Options{Port: port, Log: logger.Logger})
	live.bind(ctx, logger, srv)
	go func() {
		err := srv.Serve(ctx)
		if err != nil && ctx.Err() == nil {
			logger.Error("El panel local no está disponible", "error", err.Error(), "port", port)
		}
	}()
	logger.Info("Panel local a la escucha", "url", "http://127.0.0.1:"+strconv.Itoa(port))
}

func openPanel() int {
	reused, url, user, role, err := panel.RequestShow("")
	if system.FocusBrowserForPanel() || reused {
		fmt.Fprintln(os.Stderr, "El panel ya estaba abierto; se ha traído al frente.")
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	if url == "" {
		return 0
	}
	if err := system.OpenURL(url); err != nil {
		fmt.Fprintf(os.Stderr, "No se pudo abrir el navegador: %v\n", err)
	}
	fmt.Fprintf(os.Stderr, "Abriendo el panel como %s (%s).\nSi el navegador no se abre, copia esta dirección antes de un minuto:\n%s\n", user, role, url)
	return 0
}
