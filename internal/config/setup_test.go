package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateWritesANewConfigAndRefusesToReplaceIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("panel:\n  port: 47801\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if PeekPort(path) != 47801 {
		t.Fatalf("puerto: %d", PeekPort(path))
	}
	cfg := Config{
		Agent:      Agent{Name: "PC-NUEVO"},
		Backup:     Backup{Sources: []string{`C:\Datos`}, UseVSS: true},
		Repository: Repository{Path: `E:\Copias`, PasswordEncrypted: "dpapi:YmxvYg=="},
		Schedule:   Schedule{Times: []string{"13:30"}, Weekdays: []int{1, 2, 3, 4, 5, 6, 7}, MaxRetries: 2, RetryDelayMinutes: 15, StaleAfterDays: 2, RunMissedOnStartup: true},
		Retention:  Retention{KeepDaily: 7, KeepWeekly: 4, KeepMonthly: 12, Prune: true},
		Verify:     Verify{EveryDays: 7, ReadDataSubset: "5%"},
	}
	if err := Create(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Agent.Name != "PC-NUEVO" || loaded.Repository.PasswordEncrypted != "dpapi:YmxvYg==" || loaded.Panel.Port != 47801 || loaded.Schedule.Weekdays != nil {
		t.Fatalf("creada: %+v", loaded)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "clave-plana") {
		t.Fatal("la contraseña en claro no puede guardarse")
	}
	if err := Create(path, cfg); err == nil {
		t.Fatal("no debe sustituir una configuración válida")
	}
}

func TestCreateRejectsAnInvalidDraft(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := Create(path, Config{Agent: Agent{Name: "PC"}})
	if err == nil {
		t.Fatal("un borrador vacío debería rechazarse")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatal("un borrador inválido no debe crear el fichero")
	}
}
