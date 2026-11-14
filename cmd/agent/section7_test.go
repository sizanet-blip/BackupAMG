package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"backup-agent/internal/logging"
	"backup-agent/internal/paths"
	"backup-agent/internal/report"
	"backup-agent/internal/secrets"
)

func TestMissingDestinationRetriesAndWritesReports(t *testing.T) {
	data := t.TempDir()
	t.Setenv("BACKUP_AGENT_DATA_DIR", data)
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	password, err := secrets.Protect("clave-de-prueba")
	if err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	letter := unusedDrive(t)
	config := "agent:\n  name: PRUEBA\nbackup:\n  sources:\n    - '" + source + "'\n  use_vss: false\nrepository:\n  path: '" + letter + "\\Copias'\n  password_encrypted: \"" + password + "\"\nschedule:\n  times: [\"23:59\"]\n  run_missed_on_startup: false\n  max_retries: 2\n  retry_delay_minutes: 1\n"
	if err := os.WriteFile(paths.ConfigPath(), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	placeResticBesideTest(t)

	previous := retryWait
	retryWait = func(int) time.Duration { return 20 * time.Millisecond }
	defer func() { retryWait = previous }()

	logger, err := logging.New(logging.Options{LogFile: paths.LogFile(), Console: false})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	code := runWithRetries(context.Background(), logger, report.TriggerManual)
	if code != 1 {
		t.Fatalf("código = %d, se esperaba un fallo", code)
	}

	entries, err := os.ReadDir(paths.ReportsDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("informes = %d, se esperaban 3 (intento y dos reintentos)", len(entries))
	}
	for i, entry := range entries {
		body, err := os.ReadFile(filepath.Join(paths.ReportsDir(), entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Status  string   `json:"status"`
			Trigger string   `json:"trigger"`
			Attempt int      `json:"attempt"`
			Message string   `json:"message"`
			Errors  []string `json:"errors"`
		}
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatal(err)
		}
		if doc.Status != "failed" || !strings.Contains(doc.Message, "Destino no disponible") {
			t.Fatalf("informe %s: %+v", entry.Name(), doc)
		}
		wantTrigger := report.TriggerRetry
		wantAttempt := i + 1
		if i == 0 {
			wantTrigger = report.TriggerManual
		}
		if doc.Trigger != wantTrigger || doc.Attempt != wantAttempt {
			t.Fatalf("informe %d: trigger %s intento %d", i, doc.Trigger, doc.Attempt)
		}
	}
}

func unusedDrive(t *testing.T) string {
	t.Helper()
	for letter := byte('Z'); letter >= 'D'; letter-- {
		name := string(letter) + ":"
		if _, err := os.Stat(name + `\`); err != nil {
			return name
		}
	}
	t.Fatal("no hay una letra de unidad libre")
	return ""
}

func placeResticBesideTest(t *testing.T) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join("..", "..", "bin", "restic.exe")
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(self), "restic.exe"), body, 0o755); err != nil {
		t.Fatal(err)
	}
}
