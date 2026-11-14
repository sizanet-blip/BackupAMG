package diagnose

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedactRemovesThePassword(t *testing.T) {
	raw := []byte("repository:\n  path: 'E:\\Copias'\n  password_encrypted: \"dpapi:YmxvYg==\"\n")
	clean, err := RedactConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	text := string(clean)
	if strings.Contains(text, "YmxvYg==") || strings.Contains(strings.ToLower(text), "dpapi:") {
		t.Fatalf("el secreto sigue en la configuración: %s", text)
	}
	if !strings.Contains(text, `E:\Copias`) && !strings.Contains(text, `E:\\Copias`) {
		t.Fatalf("se perdió el destino: %s", text)
	}
	line := Redact(`time=2026-11-14T00:00:00+01:00 level=INFO msg="clave" password_encrypted="dpapi:YmxvYg==" RESTIC_PASSWORD=secreta`)
	if strings.Contains(line, "YmxvYg==") || strings.Contains(line, "secreta") {
		t.Fatalf("el registro conserva el secreto: %s", line)
	}
}

func TestTailFiltersTheLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.log")
	body := "" +
		"time=2026-11-14T00:00:00+01:00 level=INFO msg=\"arranque\"\n" +
		"time=2026-11-14T00:00:01+01:00 level=ERROR msg=\"fallo\" password_encrypted=\"dpapi:YmxvYg==\"\n" +
		"time=2026-11-14T00:00:02+01:00 level=WARN msg=\"aviso\"\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, err := Tail(path, "error", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || !strings.Contains(lines[0], "fallo") || strings.Contains(lines[0], "YmxvYg==") {
		t.Fatalf("líneas: %#v", lines)
	}
	if _, err := Tail(path, "fatal", 10); err == nil {
		t.Fatal("un nivel desconocido tendría que rechazarse")
	}
}

func TestBundleOmitsThePassword(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	secret := "dpapi:YmxvYg=="
	raw := "agent:\n  name: PRUEBA\nrepository:\n  path: 'E:\\Copias'\n  password_encrypted: \"" + secret + "\"\n"
	if err := os.WriteFile(configPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	reports := filepath.Join(dir, "reports")
	logs := filepath.Join(dir, "logs")
	if err := os.MkdirAll(reports, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reports, "2026-11-14_00-00-00.000.json"), []byte(`{"status":"success","password":"dpapi:YmxvYg=="}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reports, "nota.txt"), []byte(secret), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "agent.log"), []byte("level=INFO msg=ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err := WriteBundle(&buf, Files{Config: configPath, ReportsDir: reports, LogsDir: logs}, About{
		AgentVersion: "0.1.0", ResticVersion: "0.19.1", DataDir: dir, PanelPort: 47800,
	})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(buf.Bytes(), []byte("YmxvYg==")) || bytes.Contains(bytes.ToLower(buf.Bytes()), []byte("dpapi:")) {
		t.Fatal("el paquete incluye la contraseña")
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	for _, want := range []string{"acerca.txt", "config.yaml", "informes/2026-11-14_00-00-00.000.json", "logs/agent.log"} {
		if !names[want] {
			t.Fatalf("falta %s en %v", want, names)
		}
	}
	if names["informes/nota.txt"] {
		t.Fatal("entró un fichero que no es un informe")
	}
}
