package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateKeepsThePasswordAndRejectsAnInvalidChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := []byte("agent:\n  name: PRUEBA\nbackup:\n  sources:\n    - 'C:\\Datos'\n  use_vss: true\nrepository:\n  path: 'E:\\Copias'\n  password_encrypted: \"dpapi:YmxvYg==\"\nschedule:\n  times: [\"13:30\"]\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	bad := Editable{Sources: []string{"relativa"}, Times: []string{"13:30"}, StaleAfterDays: 2, KeepDaily: 7, EveryDays: 7, ReadDataSubset: "5%"}
	if err := Update(path, bad); err == nil {
		t.Fatal("una ruta relativa no puede guardarse")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatalf("el fichero cambió pese al error:\n%s", after)
	}

	ok := Editable{
		Sources:            []string{`C:\Fotos`},
		Excludes:           []string{"*.tmp"},
		Times:              []string{"08:00"},
		Weekdays:           []int{1, 2, 3, 4, 5},
		RunMissedOnStartup: true,
		MaxRetries:         1,
		RetryDelayMinutes:  10,
		StaleAfterDays:     3,
		KeepDaily:          14,
		KeepWeekly:         8,
		KeepMonthly:        24,
		KeepYearly:         5,
		Prune:              true,
		EveryDays:          3,
		ReadDataSubset:     "10%",
	}
	if err := Update(path, ok); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Repository.PasswordEncrypted != "dpapi:YmxvYg==" || !cfg.Backup.UseVSS {
		t.Fatalf("se perdió la contraseña o VSS: %+v", cfg)
	}
	if cfg.Backup.Sources[0] != `C:\Fotos` || cfg.Schedule.Times[0] != "08:00" || len(cfg.Schedule.Weekdays) != 5 {
		t.Fatalf("no se aplicó el cambio: %+v", cfg)
	}
	if cfg.Retention.KeepYearly != 5 || cfg.Verify.ReadDataSubset != "10%" {
		t.Fatalf("retención o verificación: %+v %+v", cfg.Retention, cfg.Verify)
	}
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(text), "clave-en-claro") {
		t.Fatal(text)
	}
}
