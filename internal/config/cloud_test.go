package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoCloudSectionBehavesAsBefore(t *testing.T) {
	cfg, err := parseWith(t, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Cloud.ServerURL != "" || cfg.Cloud.SendPaths {
		t.Fatalf("sin sección cloud no debe haber nada: %+v", cfg.Cloud)
	}
}

func TestParseCloudSection(t *testing.T) {
	cfg, err := parseWith(t, "cloud:\n  server_url: https://panel.amg.desarrollo/\n  send_paths: true\n")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Cloud.ServerURL != "https://panel.amg.desarrollo" || !cfg.Cloud.SendPaths {
		t.Fatalf("%+v", cfg.Cloud)
	}
	for _, bad := range []string{"http://panel.ejemplo.com", "panel.ejemplo.com", "https://u:p@x.com", "ftp://x"} {
		if _, err := parseWith(t, "cloud:\n  server_url: "+bad+"\n"); err == nil {
			t.Fatalf("%q debía rechazarse", bad)
		}
	}
	if _, err := parseWith(t, "cloud:\n  server_url: http://127.0.0.1:8080\n"); err != nil {
		t.Fatalf("http a la propia máquina debe valer para pruebas: %v", err)
	}
	if _, err := parseWith(t, "cloud:\n  desconocido: 1\n"); err == nil {
		t.Fatal("un campo desconocido debía rechazarse")
	}
}

// Guardar Ajustes no puede borrar la conexión con la plataforma.
func TestUpdateKeepsTheCloudSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(replicaBase+"cloud:\n  server_url: https://panel.amg.desarrollo\n  send_paths: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	edit := before.View()
	edit.KeepDaily = 9
	if err := Update(path, edit); err != nil {
		t.Fatal(err)
	}
	after, err := Load(path)
	if err != nil || after.Cloud != before.Cloud || after.Retention.KeepDaily != 9 {
		t.Fatalf("se perdió la conexión: %+v %v", after.Cloud, err)
	}
}

func TestSetCloudKeepsTheRestAndRemovesTheSectionWhenEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(replicaBase+"# nota\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetCloud(path, Cloud{ServerURL: " https://panel.amg.desarrollo/ "}); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil || cfg.Cloud.ServerURL != "https://panel.amg.desarrollo" || cfg.Repository.PasswordEncrypted != "dpapi:YmxvYg==" {
		t.Fatalf("%+v %v", cfg, err)
	}
	// Cambiar destino o copias adicionales no la borra.
	if err := SetDestination(path, `D:\Otros`, ""); err != nil {
		t.Fatal(err)
	}
	if err := SetReplicas(path, []Replica{{ID: "usb", Type: ReplicaLocal, Path: `F:\x`, Enabled: true, When: ReplicaAfterBackup}}); err != nil {
		t.Fatal(err)
	}
	cfg, _ = Load(path)
	if cfg.Cloud.ServerURL == "" || len(cfg.Replicas) != 1 {
		t.Fatalf("se perdió algo: %+v", cfg)
	}
	before, _ := os.ReadFile(path)
	if err := SetCloud(path, Cloud{ServerURL: "http://remoto.example.com"}); err == nil {
		t.Fatal("http remoto debía rechazarse")
	}
	if after, _ := os.ReadFile(path); string(before) != string(after) {
		t.Fatal("el fichero cambió pese al error")
	}
	if err := SetCloud(path, Cloud{}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "cloud:") {
		t.Fatalf("la sección debía quitarse:\n%s", data)
	}
}
