package restic

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackupRoundTrip(t *testing.T) {
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
	initResult, err := Init(ctx, InitRequest{Executable: exe, Repository: repo, Password: "clave-de-prueba"})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if initResult.AlreadyExists || initResult.ID == "" {
		t.Fatalf("init: %+v", initResult)
	}
	again, err := Init(ctx, InitRequest{Executable: exe, Repository: repo, Password: "clave-de-prueba"})
	if err != nil || !again.AlreadyExists {
		t.Fatalf("segunda init: %+v %v", again, err)
	}

	out, err := Backup(ctx, BackupRequest{
		Executable: exe,
		Repository: repo,
		Password:   "clave-de-prueba",
		Sources:    []string{source},
		Host:       "PC-PRUEBA",
		UseVSS:     false,
	})
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if out.Status != StatusSuccess || out.ExitCode != 0 || out.SnapshotID == "" {
		t.Fatalf("copia: %+v", out)
	}
	if out.FilesNew < 1 {
		t.Fatalf("no contó el fichero nuevo: %+v", out)
	}
	if out.ResticVersion == "" || strings.Contains(out.Text(), "clave-de-prueba") {
		t.Fatalf("versión o secreto: %+v", out)
	}
}
