package main

import (
	"testing"
	"time"

	"backup-agent/internal/config"
	"backup-agent/internal/paths"
	"backup-agent/internal/state"
)

func TestAutomaticVerificationWaitsForTheInterval(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BACKUP_AGENT_DATA_DIR", dir)
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Verify: config.Verify{EveryDays: 7}}
	now := time.Date(2026, 11, 13, 21, 0, 0, 0, time.Local)

	due, err := verificationDue(cfg, now)
	if err != nil || !due {
		t.Fatalf("sin verificación previa debe tocar: %v %v", due, err)
	}

	saveVerify(t, now.AddDate(0, 0, -6))
	due, err = verificationDue(cfg, now)
	if err != nil || due {
		t.Fatalf("a los 6 días no toca: %v %v", due, err)
	}

	saveVerify(t, now.AddDate(0, 0, -7))
	due, err = verificationDue(cfg, now)
	if err != nil || !due {
		t.Fatalf("a los 7 días toca: %v %v", due, err)
	}
}

func saveVerify(t *testing.T, at time.Time) {
	t.Helper()
	err := state.Save(paths.StatePath(), state.State{
		SchemaVersion: 1,
		LastVerify: &state.LastVerify{
			FinishedAt: at.Format(time.RFC3339),
			Status:     "success",
			Report:     "anterior.json",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}
