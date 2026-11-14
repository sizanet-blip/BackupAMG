package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSavePasswordPreservesConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := []byte(`# Configuración del agente
agent:
  name: "PC-1" # equipo
backup:
  sources:
    - "C:\\Datos"
repository:
  path: "E:\\Copias"
  # La rellena set-password.
  password_encrypted: ""
schedule:
  times: ["13:30"]
`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	const password = "una-clave-secreta"
	calls := 0
	err := SavePassword(path, func(label string) (string, error) {
		calls++
		if !strings.Contains(label, "Contraseña") && !strings.Contains(label, "Repite") {
			t.Fatalf("texto inesperado: %q", label)
		}
		return password, nil
	})
	if err != nil {
		t.Fatalf("SavePassword: %v", err)
	}
	if calls != 2 {
		t.Fatalf("se pidió la contraseña %d veces", calls)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), password) {
		t.Fatal("la contraseña en claro quedó en el fichero")
	}
	if !strings.Contains(string(body), "Configuración del agente") {
		t.Fatalf("se perdió el comentario de cabecera:\n%s", body)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v\n%s", err, body)
	}
	if cfg.Agent.Name != "PC-1" || cfg.Repository.Path != `E:\Copias` || len(cfg.Backup.Sources) != 1 {
		t.Fatalf("la configuración cambió: %+v", cfg)
	}
	if !strings.HasPrefix(cfg.Repository.PasswordEncrypted, "dpapi:") {
		t.Fatalf("valor guardado: %s", cfg.Repository.PasswordEncrypted)
	}
}

func TestSavePasswordRejectsMismatchAndEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := []byte("repository:\n  path: \"E:\\\\Copias\"\n  password_encrypted: \"\"\n")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}

	err := SavePassword(path, func(label string) (string, error) {
		if strings.HasPrefix(label, "Repite") {
			return "otra", nil
		}
		return "una", nil
	})
	if err == nil || !strings.Contains(err.Error(), "no coinciden") {
		t.Fatalf("mismatch: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatal("un fallo no debe modificar el fichero")
	}

	err = SavePassword(path, func(string) (string, error) { return "", nil })
	if err == nil || !strings.Contains(err.Error(), "vacía") {
		t.Fatalf("vacía: %v", err)
	}
}

func TestParseRejectsPlaintextPassword(t *testing.T) {
	_, err := Parse([]byte(`
backup:
  sources: ["C:\\Datos"]
repository:
  path: "E:\\Copias"
  password_encrypted: "temporal"
schedule:
  times: ["13:30"]
`))
	if err == nil || !strings.Contains(err.Error(), "no está cifrado") {
		t.Fatalf("texto en claro: %v", err)
	}
}
