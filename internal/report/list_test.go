package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListFiltersAndHidesThePassword(t *testing.T) {
	dir := t.TempDir()
	writeReport(t, dir, "2026-11-13_23-06-29.103", `{
		"schema_version": 1, "type": "backup", "status": "success",
		"started_at": "2026-11-13T23:06:29+01:00", "finished_at": "2026-11-13T23:06:31+01:00",
		"duration_seconds": 2, "data_added_bytes": 19713, "password": "no-debe-salir"
	}`)
	writeReport(t, dir, "2026-11-12_20-00-00.000", `{
		"schema_version": 1, "type": "verify", "status": "failed",
		"started_at": "2026-11-12T20:00:00+01:00", "finished_at": "2026-11-12T20:01:00+01:00",
		"duration_seconds": 60, "data_added_bytes": 0
	}`)
	if err := os.WriteFile(filepath.Join(dir, "nota.json"), []byte(`{"password":"no-debe-salir"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	all, err := List(dir, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].ID != "2026-11-13_23-06-29.103" || all[0].Status != "success" {
		t.Fatalf("lista: %+v", all)
	}
	failed, err := ParseFilter("failed", "2026-11-12", "2026-11-12")
	if err != nil {
		t.Fatal(err)
	}
	only, err := List(dir, failed)
	if err != nil || len(only) != 1 || only[0].Type != "verify" {
		t.Fatalf("filtro: %+v %v", only, err)
	}
	day, err := ParseFilter("", "2026-11-13", "2026-11-13")
	if err != nil {
		t.Fatal(err)
	}
	today, err := List(dir, day)
	if err != nil || len(today) != 1 || today[0].DataAddedBytes != 19713 {
		t.Fatalf("día: %+v %v", today, err)
	}
	if _, err := ParseFilter("rota", "", ""); err == nil {
		t.Fatal("un resultado desconocido debería rechazarse")
	}
	if _, err := ParseFilter("", "2026-11-14", "2026-11-13"); err == nil {
		t.Fatal("un rango invertido debería rechazarse")
	}

	rep, err := Open(dir, all[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Status != "success" || strings.Contains(string(encoded), "no-debe-salir") || strings.Contains(string(encoded), "password") {
		t.Fatalf("detalle: %s", encoded)
	}
	if _, err := Open(dir, `..\secreto`); err == nil {
		t.Fatal("una ruta no puede abrirse")
	}
	empty, err := List(filepath.Join(dir, "no-existe"), Filter{})
	if err != nil || len(empty) != 0 {
		t.Fatalf("vacío: %+v %v", empty, err)
	}
}

func writeReport(t *testing.T, dir, id, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
