package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildKeepsSchemaStable(t *testing.T) {
	started := time.Date(2026, 11, 13, 13, 30, 0, 0, time.FixedZone("CET", 3600))
	rep := Build(Input{
		AgentVersion:         "0.1.0",
		ResticVersion:        "0.19.1",
		Hostname:             "PC-OFICINA-01",
		Started:              started,
		Finished:             started.Add(4*time.Minute + 12*time.Second),
		Status:               "success",
		Trigger:              TriggerManual,
		Attempt:              1,
		Sources:              []string{`C:\Datos`},
		Repository:           `E:\Copias`,
		VSSUsed:              true,
		FilesNew:             12,
		FilesChanged:         30,
		FilesUnmodified:      15400,
		DataAddedBytes:       52428800,
		TotalFilesProcessed:  15442,
		TotalBytesProcessed:  8589934592,
		SnapshotID:           "abc123",
		DestinationFreeBytes: 107374182400,
		Message:              "Copia completada correctamente",
	})
	if rep.SchemaVersion != 1 || rep.Type != TypeBackup {
		t.Fatalf("cabecera: %+v", rep)
	}
	if rep.DurationSeconds != 252 {
		t.Fatalf("duración: %d", rep.DurationSeconds)
	}
	if rep.StartedAt != "2026-11-13T13:30:00+01:00" || rep.FinishedAt != "2026-11-13T13:34:12+01:00" {
		t.Fatalf("fechas: %s %s", rep.StartedAt, rep.FinishedAt)
	}
	if rep.Retention.Applied || rep.Retention.SnapshotsRemoved != 0 {
		t.Fatalf("la retención aún no se aplica: %+v", rep.Retention)
	}
	raw, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"schema_version", "type", "agent_version", "restic_version", "hostname",
		"started_at", "finished_at", "duration_seconds", "status", "trigger", "attempt",
		"sources", "repository", "vss_used", "files_new", "files_changed", "files_unmodified",
		"data_added_bytes", "total_files_processed", "total_bytes_processed", "snapshot_id",
		"destination_free_bytes", "skipped_files", "errors", "retention", "message",
	} {
		if _, ok := doc[key]; !ok {
			t.Fatalf("falta el campo %s", key)
		}
	}
	if doc["skipped_files"] == nil || doc["errors"] == nil {
		t.Fatal("las listas vacías no deben ser null")
	}
	if strings.Contains(string(raw), "clave") {
		t.Fatal("el informe no debe hablar de la contraseña")
	}
}

func TestBuildFailedIncludesError(t *testing.T) {
	rep := Build(Input{
		Status:   "failed",
		Trigger:  TriggerManual,
		Errors:   []string{"destino no disponible: conecta el disco USB"},
		Message:  "destino no disponible: conecta el disco USB",
		Hostname: "PC-1",
	})
	if rep.Status != "failed" || len(rep.Errors) != 1 || rep.SnapshotID != "" {
		t.Fatalf("%+v", rep)
	}
	if rep.Attempt != 1 {
		t.Fatalf("intento: %d", rep.Attempt)
	}
}

func TestWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	started := time.Date(2026, 11, 13, 20, 44, 19, 123000000, time.Local)
	rep := Build(Input{
		AgentVersion: "0.1.0",
		Hostname:     "PC-PRUEBA",
		Started:      started,
		Finished:     started.Add(2 * time.Second),
		Status:       "warning",
		Trigger:      TriggerManual,
		Sources:      []string{`C:\Datos`},
		Repository:   `E:\Copias`,
		SkippedFiles: []SkippedFile{{Path: `C:\Datos\a.pst`, Reason: "en uso"}},
		Message:      "Copia completada con avisos",
	})
	path, err := Write(dir, rep, started)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "2026-11-13_20-44-19.123.json" {
		t.Fatalf("nombre: %s", path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got Report
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "warning" || got.SkippedFiles[0].Path != `C:\Datos\a.pst` {
		t.Fatalf("%+v", got)
	}
	if strings.Contains(filepath.Base(path), ":") {
		t.Fatalf("el nombre no es válido en Windows: %s", path)
	}
}
