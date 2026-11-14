package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"backup-agent/internal/config"
	"backup-agent/internal/paths"
	"backup-agent/internal/restic"
	"backup-agent/internal/secrets"
)

// destinationFixture deja una configuración válida en una carpeta de datos de prueba.
func destinationFixture(t *testing.T, password string) (oldPath, exe string) {
	t.Helper()
	t.Setenv("BACKUP_AGENT_DATA_DIR", t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	protected, err := secrets.Protect(password)
	if err != nil {
		t.Fatal(err)
	}
	oldPath = filepath.Join(t.TempDir(), "destino-anterior")
	source := t.TempDir()
	body := "agent:\n  name: PRUEBA\nbackup:\n  sources:\n    - '" + source + "'\n  use_vss: false\nrepository:\n  path: '" + oldPath + "'\n  password_encrypted: \"" + protected + "\"\nschedule:\n  times: [\"23:59\"]\n  run_missed_on_startup: false\n  max_retries: 1\n  retry_delay_minutes: 1\n"
	if err := os.WriteFile(paths.ConfigPath(), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	// restic.exe se copia junto al test una sola vez: reescribirlo mientras otro test lo usa daría fallos sueltos.
	if self, selfErr := os.Executable(); selfErr == nil {
		if _, statErr := os.Stat(filepath.Join(filepath.Dir(self), "restic.exe")); statErr != nil {
			placeResticBesideTest(t)
		}
	} else {
		placeResticBesideTest(t)
	}
	exe, err = restic.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return oldPath, exe
}

func saveDestinationBody(t *testing.T, path string) []string {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		t.Fatal(err)
	}
	problems, err := saveDestination(raw)
	if err != nil {
		t.Fatal(err)
	}
	return problems
}

func currentRepositoryPath(t *testing.T) string {
	t.Helper()
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Repository.Path
}

func TestNewEmptyDestinationIsPreparedBeforeSaving(t *testing.T) {
	oldPath, _ := destinationFixture(t, "clave-de-prueba")
	target := filepath.Join(t.TempDir(), "Copias")
	if problems := saveDestinationBody(t, target); len(problems) != 0 {
		t.Fatalf("problemas inesperados: %v", problems)
	}
	if !resticRepo(target) {
		t.Fatal("la carpeta nueva debería haberse preparado como repositorio")
	}
	if got := currentRepositoryPath(t); !strings.EqualFold(got, target) {
		t.Fatalf("destino guardado = %s, se esperaba %s", got, target)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("el destino anterior no debe tocarse: no existía y sigue sin existir")
	}
}

func TestExistingRepositoryWithTheSamePasswordIsAccepted(t *testing.T) {
	_, exe := destinationFixture(t, "clave-de-prueba")
	target := filepath.Join(t.TempDir(), "Copias")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if _, err := restic.Init(ctx, restic.InitRequest{Executable: exe, Repository: target, Password: "clave-de-prueba"}); err != nil {
		t.Fatal(err)
	}
	if problems := saveDestinationBody(t, target); len(problems) != 0 {
		t.Fatalf("problemas inesperados: %v", problems)
	}
	if got := currentRepositoryPath(t); !strings.EqualFold(got, target) {
		t.Fatalf("destino guardado = %s, se esperaba %s", got, target)
	}
}

func TestExistingRepositoryWithAnotherPasswordIsRejectedAndConfigStays(t *testing.T) {
	oldPath, exe := destinationFixture(t, "clave-de-prueba")
	target := filepath.Join(t.TempDir(), "Copias")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if _, err := restic.Init(ctx, restic.InitRequest{Executable: exe, Repository: target, Password: "otra-clave-distinta"}); err != nil {
		t.Fatal(err)
	}
	problems := saveDestinationBody(t, target)
	if len(problems) == 0 || !strings.Contains(problems[0], "otra contraseña") || !strings.Contains(problems[0], "no se ha cambiado") {
		t.Fatalf("debería rechazarse con un mensaje claro: %v", problems)
	}
	if got := currentRepositoryPath(t); !strings.EqualFold(got, oldPath) {
		t.Fatalf("la configuración en uso no debe cambiar: %s", got)
	}
}

func TestSameDestinationIsNotPreparedAgain(t *testing.T) {
	oldPath, _ := destinationFixture(t, "clave-de-prueba")
	// Mismo destino: no hay nada que preparar y se guarda como siempre.
	if problems := saveDestinationBody(t, oldPath); len(problems) != 0 {
		t.Fatalf("problemas inesperados: %v", problems)
	}
	if resticRepo(oldPath) {
		t.Fatal("no se debe crear un repositorio si el destino no ha cambiado")
	}
}
