package restic

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestChangePasswordRoundTrip(t *testing.T) {
	exe := filepath.Join("..", "..", "bin", "restic.exe")
	if _, err := os.Stat(exe); err != nil {
		t.Skip("no está bin/restic.exe")
	}
	repo := filepath.Join(t.TempDir(), "repo")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	oldPassword := "clave-de-prueba"
	newPassword := "clave-nueva"
	if _, err := Init(ctx, InitRequest{Executable: exe, Repository: repo, Password: oldPassword}); err != nil {
		t.Fatal(err)
	}
	ok, exists, err := PasswordOK(ctx, exe, repo, oldPassword)
	if err != nil || !ok || !exists {
		t.Fatalf("contraseña inicial: %v %v %v", ok, exists, err)
	}
	if err := ChangePassword(ctx, exe, repo, oldPassword, newPassword); err != nil {
		t.Fatal(err)
	}
	ok, _, err = PasswordOK(ctx, exe, repo, newPassword)
	if err != nil || !ok {
		t.Fatalf("contraseña nueva: %v %v", ok, err)
	}
	ok, _, err = PasswordOK(ctx, exe, repo, oldPassword)
	if err != nil || ok {
		t.Fatalf("la contraseña anterior todavía abre: %v %v", ok, err)
	}
	missing, exists, err := PasswordOK(ctx, exe, filepath.Join(t.TempDir(), "vacio"), oldPassword)
	if err != nil || missing || exists {
		t.Fatalf("sin repositorio: %v %v %v", missing, exists, err)
	}
	if strings.Contains(oldPassword+newPassword, "\n") {
		t.Fatal("las claves de prueba no pueden tener un salto de línea")
	}
}
