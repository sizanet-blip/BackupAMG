package restic

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRestoreRoundTrip(t *testing.T) {
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
	if _, err := Init(ctx, InitRequest{Executable: exe, Repository: repo, Password: "clave-de-prueba"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Backup(ctx, BackupRequest{
		Executable: exe, Repository: repo, Password: "clave-de-prueba",
		Sources: []string{source}, Host: "PC-PRUEBA",
	}); err != nil {
		t.Fatal(err)
	}
	snaps, err := Snapshots(ctx, exe, repo, "clave-de-prueba")
	if err != nil || len(snaps) != 1 || snaps[0].ID == "" {
		t.Fatalf("copias: %+v %v", snaps, err)
	}
	children, err := ListChildren(ctx, exe, repo, "clave-de-prueba", snaps[0].ID, source)
	if err != nil || len(children) != 1 || children[0].Name != "hola.txt" {
		t.Fatalf("contenido: %+v %v", children, err)
	}
	found, err := Find(ctx, exe, repo, "clave-de-prueba", snaps[0].ID, "hola.txt")
	if err != nil || len(found) != 1 || found[0].Path != children[0].Path {
		t.Fatalf("búsqueda: %+v %v", found, err)
	}
	target := filepath.Join(root, "recuperado")
	result, err := Restore(ctx, exe, repo, "clave-de-prueba", snaps[0].ID, target, []string{children[0].Path}, false, nil)
	if err != nil || result.Files < 1 {
		t.Fatalf("restaurar: %+v %v", result, err)
	}
	gotPath := ""
	err = filepath.WalkDir(target, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr == nil && entry != nil && entry.Name() == "hola.txt" {
			gotPath = path
		}
		return nil
	})
	if err != nil || gotPath == "" {
		t.Fatalf("no se encontró el fichero recuperado: %v", err)
	}
	got, err := os.ReadFile(gotPath)
	if err != nil || string(got) != "hola" {
		t.Fatalf("fichero: %s %v", got, err)
	}
}
