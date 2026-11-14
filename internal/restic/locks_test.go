package restic

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsStaleOnlyForThisComputer(t *testing.T) {
	now := time.Date(2026, 11, 13, 21, 0, 0, 0, time.UTC)
	fresh := fileLock{Time: now.Add(-time.Minute), Hostname: "PC-1", PID: 10}
	alive := func(pid int) bool { return pid == 10 }
	if IsStale(fresh, "PC-1", now, alive) {
		t.Fatal("un restic en marcha no es un bloqueo antiguo")
	}
	dead := fresh
	dead.PID = 11
	if !IsStale(dead, "PC-1", now, alive) {
		t.Fatal("el proceso ya no existe")
	}
	other := fresh
	other.Hostname = "otro-pc"
	other.PID = 99
	if IsStale(other, "PC-1", now, alive) {
		t.Fatal("no se toca el bloqueo de otro equipo")
	}
	old := fresh
	old.Time = now.Add(-31 * time.Minute)
	if !IsStale(old, "pc-1", now, alive) {
		t.Fatal("un bloqueo de más de 30 minutos está obsoleto")
	}
}

func TestStaleLockIDs(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, "locks")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	writeLock(t, dir, "antiguo", fileLock{Time: now.Add(-time.Minute), Hostname: "PC-1", PID: 5})
	writeLock(t, dir, "vivo", fileLock{Time: now, Hostname: "PC-1", PID: 8})
	ids, err := StaleLockIDs(repo, "PC-1", now, func(pid int) bool { return pid == 8 })
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "antiguo" {
		t.Fatalf("ids: %v", ids)
	}
}

func writeLock(t *testing.T, dir, name string, lock fileLock) {
	t.Helper()
	body := []byte(`{"time":"` + lock.Time.Format(time.RFC3339) + `","hostname":"` + lock.Hostname + `","pid":` + itoa(lock.PID) + `}`)
	if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
		t.Fatal(err)
	}
}
