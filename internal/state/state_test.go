package state

import (
	"path/filepath"
	"testing"
)

func TestLoadMissingIsEmpty(t *testing.T) {
	st, err := Load(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if st.SchemaVersion != 1 || st.LastBackup != nil {
		t.Fatalf("%+v", st)
	}
}

func TestSaveAndLoadLastBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st := State{LastBackup: &LastBackup{
		FinishedAt: "2026-11-13T20:44:21+01:00",
		Status:     "success",
		SnapshotID: "abc",
		Report:     "2026-11-13_20-44-19.000.json",
	}}
	if err := Save(path, st); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastBackup == nil || got.LastBackup.SnapshotID != "abc" || got.LastBackup.Report != st.LastBackup.Report {
		t.Fatalf("%+v", got.LastBackup)
	}
}
