package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetDestinationKeepsThePassword(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := []byte("" +
		"# aviso de la contraseña\n" +
		"agent:\n  name: PRUEBA\n" +
		"backup:\n  sources:\n    - 'C:\\Datos'\n  use_vss: true\n" +
		"repository:\n  path: 'E:\\Copias'\n  password_encrypted: \"dpapi:YmxvYg==\"\n" +
		"schedule:\n  times: [\"13:30\"]\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := SetDestination(path, "relativa", ""); err == nil {
		t.Fatal("una ruta relativa debería rechazarse")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("un destino inválido modificó el fichero")
	}

	id := `\\?\Volume{12345678-1234-1234-1234-123456789abc}\`
	if err := SetDestination(path, `E:\Copias`, id); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Repository.Path != `E:\Copias` || cfg.Repository.VolumeID != id || cfg.Repository.PasswordEncrypted != "dpapi:YmxvYg==" || !cfg.Backup.UseVSS {
		t.Fatalf("destino: %+v vss=%v", cfg.Repository, cfg.Backup.UseVSS)
	}
	kept, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(kept), "aviso de la contraseña") {
		t.Fatal("se perdió el aviso de la contraseña")
	}

	if err := SetDestination(path, `\\nas\copias`, ""); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Repository.Path != `\\nas\copias` || cfg.Repository.VolumeID != "" || cfg.Repository.PasswordEncrypted != "dpapi:YmxvYg==" {
		t.Fatalf("red: %+v", cfg.Repository)
	}
}

func TestUpdateKeepsTheVolumeID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	id := `\\?\Volume{12345678-1234-1234-1234-123456789abc}\`
	body := "agent:\n  name: PRUEBA\nbackup:\n  sources:\n    - 'C:\\Datos'\nrepository:\n  path: 'E:\\Copias'\n  volume_id: '" + id + "'\n  password_encrypted: \"dpapi:YmxvYg==\"\nschedule:\n  times: [\"13:30\"]\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	edit := cfg.View()
	edit.Times = []string{"08:00"}
	if err := Update(path, edit); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Repository.VolumeID != id || cfg.Repository.PasswordEncrypted != "dpapi:YmxvYg==" || cfg.Schedule.Times[0] != "08:00" {
		t.Fatalf("se perdió el disco o la contraseña: %+v", cfg.Repository)
	}
}

func TestRejectsABadVolumeID(t *testing.T) {
	raw := []byte("agent:\n  name: PRUEBA\nbackup:\n  sources:\n    - 'C:\\Datos'\nrepository:\n  path: 'E:\\Copias'\n  volume_id: \"E:\"\n  password_encrypted: \"dpapi:YmxvYg==\"\nschedule:\n  times: [\"13:30\"]\n")
	_, err := Parse(raw)
	invalid, ok := err.(*ValidationError)
	if !ok || !strings.Contains(invalid.Error(), "volume_id") {
		t.Fatalf("identificador: %v", err)
	}
}
