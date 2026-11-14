package main

import (
	"bytes"
	"context"
	"time"

	"backup-agent/internal/app"
	"backup-agent/internal/config"
	"backup-agent/internal/diagnose"
	"backup-agent/internal/paths"
	"backup-agent/internal/restic"
)

func diagnostics() (any, error) {
	about := diagnose.About{
		AgentVersion: app.Version,
		DataDir:      paths.DataDir(),
		PanelPort:    config.PeekPort(paths.ConfigPath()),
	}
	exe, err := restic.Executable()
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		version, versionErr := restic.Version(ctx, exe)
		cancel()
		if versionErr == nil {
			about.ResticVersion = version
		}
	}
	return about, nil
}

func diagnosticLog(level string) (any, error) {
	lines, err := diagnose.Tail(paths.LogFile(), level, 200)
	if err != nil {
		return nil, err
	}
	return map[string]any{"lines": lines}, nil
}

func diagnosticBundle() ([]byte, error) {
	about, err := diagnostics()
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = diagnose.WriteBundle(&buf, diagnose.Files{
		Config:     paths.ConfigPath(),
		State:      paths.StatePath(),
		ReportsDir: paths.ReportsDir(),
		LogsDir:    paths.LogsDir(),
	}, about.(diagnose.About))
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
