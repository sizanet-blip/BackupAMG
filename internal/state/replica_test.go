package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Un state.json de antes (sin copias adicionales) se lee igual y no gana campos al guardarlo.
func TestOldStateStillLoadsAndSavesWithoutReplicas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	old := `{"schema_version":1,"last_backup":{"finished_at":"2026-11-13T20:44:21+01:00","status":"success","snapshot_id":"abc","report":"r.json"}}`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.GoodSnapshot() != "abc" || st.Replicas != nil {
		t.Fatalf("%+v", st)
	}
	if err := Save(path, st); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "replicas") {
		t.Fatalf("no debía aparecer replicas:\n%s", data)
	}
}

func TestReplicaStateRoundTripAndPending(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st := State{
		LastBackup:       &LastBackup{Status: "failed", FinishedAt: "2026-11-14T20:00:00+01:00"},
		LastGoodSnapshot: "good1",
		Replicas: map[string]*ReplicaState{
			"usb": {LastSuccessAt: "2026-11-13T21:00:00+01:00", LastSnapshot: "good1", LastStatus: ReplicaOK},
		},
	}
	if err := Save(path, st); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	// Una copia principal fallida no deja nada nuevo pendiente en la adicional.
	if got.GoodSnapshot() != "good1" || got.ReplicaPending("usb") {
		t.Fatalf("pendiente tras una copia fallida: %+v", got)
	}
	if !got.ReplicaPending("otra") {
		t.Fatal("una copia adicional nueva tiene todo pendiente")
	}
	got.LastGoodSnapshot = "good2"
	if !got.ReplicaPending("usb") {
		t.Fatal("con una copia nueva, la adicional queda pendiente")
	}
	if (State{}).ReplicaPending("usb") {
		t.Fatal("sin ninguna copia correcta no hay nada que replicar")
	}
}
