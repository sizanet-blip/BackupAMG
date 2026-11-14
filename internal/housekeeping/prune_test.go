package housekeeping

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPruneKeepsNinetyDays(t *testing.T) {
	root := t.TempDir()
	reports := filepath.Join(root, "reports")
	logs := filepath.Join(root, "logs")
	if err := os.Mkdir(reports, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.Local)
	oldReport := filepath.Join(reports, "2026-08-01_10-00-00.000.json")
	newReport := filepath.Join(reports, "2026-11-01_10-00-00.000.json")
	if err := os.WriteFile(oldReport, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newReport, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(logs, "agent.log")
	rotated := filepath.Join(logs, "agent.log.1")
	if err := os.WriteFile(active, []byte("activo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rotated, []byte("viejo"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := now.AddDate(0, 0, -100)
	if err := os.Chtimes(active, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(rotated, old, old); err != nil {
		t.Fatal(err)
	}

	n, err := Prune(reports, logs, now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("borrados = %d", n)
	}
	if _, err := os.Stat(oldReport); !os.IsNotExist(err) {
		t.Fatal("el informe antiguo sigue")
	}
	if _, err := os.Stat(newReport); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(active); err != nil {
		t.Fatal("no debe borrar el log activo")
	}
	if _, err := os.Stat(rotated); !os.IsNotExist(err) {
		t.Fatal("el log rotado antiguo sigue")
	}
}
