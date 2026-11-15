package paths

import (
	"os"
	"path/filepath"
	"testing"

	"backup-agent/internal/app"
)

func TestDataDirOverride(t *testing.T) {
	const custom = `D:\copias-prueba\datos`
	t.Setenv(app.DataDirEnv, custom)

	if got := DataDir(); got != custom {
		t.Fatalf("DataDir() = %q, se esperaba %q", got, custom)
	}
	if got := ConfigPath(); got != filepath.Join(custom, "config.yaml") {
		t.Fatalf("ConfigPath() = %q", got)
	}
	if got := StatePath(); got != filepath.Join(custom, "state.json") {
		t.Fatalf("StatePath() = %q", got)
	}
}

func TestDataDirUsesProgramData(t *testing.T) {
	t.Setenv(app.DataDirEnv, "")
	t.Setenv("ProgramData", `C:\ProgramData`)

	want := filepath.Join(`C:\ProgramData`, app.Name)
	if got := DataDir(); got != want {
		t.Fatalf("DataDir() = %q, se esperaba %q", got, want)
	}
	if got := LogFile(); got != filepath.Join(want, "logs", "agent.log") {
		t.Fatalf("LogFile() = %q", got)
	}
}

func TestEnsureCreatesDirectories(t *testing.T) {
	root := t.TempDir()
	t.Setenv(app.DataDirEnv, root)

	if err := Ensure(); err != nil {
		t.Fatalf("Ensure() devolvió error: %v", err)
	}
	for _, dir := range []string{ReportsDir(), LogsDir()} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("no existe %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s no es un directorio", dir)
		}
	}
}

func TestTempDirLivesInsideTheDataDirAndIsCleaned(t *testing.T) {
	data := t.TempDir()
	t.Setenv("BACKUP_AGENT_DATA_DIR", data)
	dir, err := MkdirTemp("ba-prueba-")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(dir) != filepath.Join(data, "tmp") {
		t.Fatalf("la carpeta temporal debe estar dentro de la carpeta de datos: %s", dir)
	}
	if err := os.WriteFile(filepath.Join(dir, "secreto"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	CleanTemp()
	if entries, _ := os.ReadDir(TempDir()); len(entries) != 0 {
		t.Fatalf("CleanTemp debía vaciar la carpeta: %v", entries)
	}
	if _, err := os.Stat(TempDir()); err != nil {
		t.Fatal("la carpeta tmp en sí debe seguir existiendo")
	}
	CleanTemp() // sin nada que limpiar, no falla
}
