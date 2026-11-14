package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"backup-agent/internal/app"
	"backup-agent/internal/status"
)

func TestPanelStatusReflectsTheLastBackup(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(filepath.Join(dir, "reports"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "datos.bin"), []byte("hola"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(app.DataDirEnv, dir)
	config := "agent:\n  name: PRUEBA\nbackup:\n  sources:\n    - 'C:\\Datos'\nrepository:\n  path: '" + repo + "'\n  password_encrypted: \"dpapi:YmxvYg==\"\nschedule:\n  times: [\"00:00\"]\n  stale_after_days: 2\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	finished := time.Now().Add(time.Minute).Format(time.RFC3339)
	stateJSON := "{\n  \"schema_version\": 1,\n  \"last_backup\": {\n    \"finished_at\": \"" + finished + "\",\n    \"status\": \"success\",\n    \"report\": \"ultima.json\"\n  }\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(stateJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	reportJSON := "{\n  \"duration_seconds\": 12,\n  \"data_added_bytes\": 4096,\n  \"message\": \"Copia correcta\"\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "reports", "ultima.json"), []byte(reportJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	raw, err := json.Marshal(live.snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(raw)), "password") || strings.Contains(string(raw), "dpapi") {
		t.Fatalf("el estado incluye la contraseña: %s", raw)
	}
	var view status.View
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	if view.Level != status.Green {
		t.Fatalf("nivel %s: %s", view.Level, raw)
	}
	if !view.Destination.Connected || !view.Destination.UsedKnown || view.Destination.UsedBytes < 4 {
		t.Fatalf("destino: %+v", view.Destination)
	}
	if view.LastBackup == nil || view.LastBackup.DurationSeconds != 12 || view.LastBackup.DataAddedBytes != 4096 {
		t.Fatalf("última copia: %+v", view.LastBackup)
	}
	if view.NextBackup == "" {
		t.Fatalf("sin próxima copia: %s", raw)
	}
}
