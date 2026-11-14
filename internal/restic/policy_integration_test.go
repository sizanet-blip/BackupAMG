package restic

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestForgetAndCheck(t *testing.T) {
	exe := filepath.Join("..", "..", "bin", "restic.exe")
	if _, err := os.Stat(exe); err != nil {
		t.Skip("no está bin/restic.exe")
	}
	root := t.TempDir()
	source := filepath.Join(root, "origen")
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "hola.txt"), []byte("hola"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	const password = "clave-retencion"
	if _, err := Init(ctx, InitRequest{Executable: exe, Repository: repo, Password: password}); err != nil {
		t.Fatal(err)
	}
	backup := BackupRequest{Executable: exe, Repository: repo, Password: password, Sources: []string{source}, Host: "PC-PRUEBA"}
	if _, err := Backup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	if _, err := Backup(ctx, backup); err != nil {
		t.Fatal(err)
	}

	kept, err := Forget(ctx, exe, repo, password, RetentionPolicy{KeepDaily: 7, KeepWeekly: 4, KeepMonthly: 12, Prune: true})
	if err != nil {
		t.Fatal(err)
	}
	if !kept.Applied || kept.SnapshotsRemoved != 0 {
		t.Fatalf("dos copias del mismo día deben conservarse: %+v", kept)
	}

	check, err := Check(ctx, exe, repo, password, "5%")
	if err != nil {
		t.Fatal(err)
	}
	if check.Status != StatusSuccess || check.NumErrors != 0 {
		t.Fatalf("verificación: %+v", check)
	}
}
