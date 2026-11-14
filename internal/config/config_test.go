package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseValidAppliesValues(t *testing.T) {
	cfg, err := Parse([]byte(`
agent:
  name: "PC-OFICINA-01"
backup:
  sources:
    - "C:\\Users\\Juan\\Documents"
    - "C:/Users/Juan/Desktop"
  excludes:
    - "*.tmp"
    - "~$*"
  use_vss: true
repository:
  path: "\\\\nas\\copias"
  password_encrypted: "dpapi:YmxvYg=="
schedule:
  times: ["9:05", "20:00"]
  run_missed_on_startup: false
  max_retries: 0
  retry_delay_minutes: 0
retention:
  keep_daily: 1
  keep_weekly: 0
  keep_monthly: 3
  prune: false
verify:
  every_days: 14
  read_data_subset: "2.5%"
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Agent.Name != "PC-OFICINA-01" {
		t.Fatalf("nombre: %q", cfg.Agent.Name)
	}
	if len(cfg.Backup.Sources) != 2 || cfg.Backup.Sources[1] != "C:/Users/Juan/Desktop" {
		t.Fatalf("orígenes: %#v", cfg.Backup.Sources)
	}
	if !cfg.Backup.UseVSS {
		t.Fatal("use_vss debería ser true")
	}
	if cfg.Repository.Path != `\\nas\copias` {
		t.Fatalf("destino: %q", cfg.Repository.Path)
	}
	if cfg.Repository.PasswordEncrypted != "dpapi:YmxvYg==" {
		t.Fatal("la contraseña cifrada no se conservó")
	}
	if strings.Join(cfg.Schedule.Times, ",") != "09:05,20:00" {
		t.Fatalf("horas: %#v", cfg.Schedule.Times)
	}
	if cfg.Schedule.RunMissedOnStartup || cfg.Schedule.MaxRetries != 0 {
		t.Fatalf("programación: %+v", cfg.Schedule)
	}
	if cfg.Retention.Prune || cfg.Retention.KeepWeekly != 0 || cfg.Retention.KeepMonthly != 3 {
		t.Fatalf("retención: %+v", cfg.Retention)
	}
	if cfg.Verify.EveryDays != 14 || cfg.Verify.ReadDataSubset != "2.5%" {
		t.Fatalf("verificación: %+v", cfg.Verify)
	}
	if cfg.Panel.Port != DefaultPanelPort {
		t.Fatalf("puerto: %d", cfg.Panel.Port)
	}
}

func TestParseDefaultsWhenOmitted(t *testing.T) {
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Parse([]byte(`
backup:
  sources:
    - "D:\\Datos"
repository:
  path: "E:\\Copias"
  password_encrypted: "dpapi:YmxvYg=="
schedule:
  times: ["13:30"]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Agent.Name != host {
		t.Fatalf("nombre = %q, se esperaba el del equipo %q", cfg.Agent.Name, host)
	}
	if !cfg.Backup.UseVSS || !cfg.Schedule.RunMissedOnStartup || !cfg.Retention.Prune {
		t.Fatalf("un valor por defecto quedó desactivado: %+v", cfg)
	}
	if cfg.Schedule.MaxRetries != 2 || cfg.Schedule.RetryDelayMinutes != 15 || cfg.Schedule.StaleAfterDays != 2 {
		t.Fatalf("reintentos: %+v", cfg.Schedule)
	}
	if cfg.Retention.KeepDaily != 7 || cfg.Retention.KeepWeekly != 4 || cfg.Retention.KeepMonthly != 12 || cfg.Retention.KeepYearly != 0 {
		t.Fatalf("retención: %+v", cfg.Retention)
	}
	if cfg.Verify.EveryDays != 7 || cfg.Verify.ReadDataSubset != "5%" {
		t.Fatalf("verificación: %+v", cfg.Verify)
	}
}

func TestParseReportsAllProblems(t *testing.T) {
	_, err := Parse([]byte(`
backup:
  sources:
    - "relativa"
    - "C:\\Datos"
    - "C:\\Datos"
  excludes:
    - ""
repository:
  path: "copias"
  password_encrypted: ""
schedule:
  times: ["25:00", "13:30", "13:30"]
  max_retries: -1
  retry_delay_minutes: 0
retention:
  keep_daily: 0
  keep_weekly: 0
  keep_monthly: 0
verify:
  every_days: 0
  read_data_subset: "abc"
`))
	var verr *ValidationError
	if !asValidation(err, &verr) {
		t.Fatalf("se esperaba ValidationError, llegó %v", err)
	}
	text := err.Error()
	for _, want := range []string{
		"backup.sources[0]",
		"backup.sources[2]",
		"backup.excludes[0]",
		"repository.path",
		"repository.password_encrypted",
		"schedule.times[0]",
		"schedule.times[2]",
		"schedule.max_retries",
		"retention: la política no conserva ninguna copia",
		"verify.every_days",
		"verify.read_data_subset",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("falta %q en:\n%s", want, text)
		}
	}
}

func TestParseRejectsUnknownFieldAndInvalidYAML(t *testing.T) {
	_, err := Parse([]byte("backup:\n  origen: C:\\Datos\n"))
	if err == nil || !strings.Contains(err.Error(), "desconocidos") {
		t.Fatalf("campo desconocido: %v", err)
	}
	_, err = Parse([]byte(":\n  - ["))
	if err == nil || !strings.Contains(err.Error(), "no es un YAML válido") {
		t.Fatalf("YAML inválido: %v", err)
	}
	_, err = Parse([]byte(""))
	if err == nil || !strings.Contains(err.Error(), "vacío") {
		t.Fatalf("vacío: %v", err)
	}
	_, err = Parse([]byte("agent:\n  name: A\n---\nagent:\n  name: B\n"))
	if err == nil || !strings.Contains(err.Error(), "más de un documento") {
		t.Fatalf("dos documentos: %v", err)
	}
}

func TestParseAcceptsReadDataSubsetFormats(t *testing.T) {
	for _, subset := range []string{"5%", "2.5%", "100%", "1/7", "10G", "500M", "50K", "1t"} {
		_, err := Parse([]byte(minimalYAML(subset)))
		if err != nil {
			t.Fatalf("subset %s: %v", subset, err)
		}
	}
	for _, subset := range []string{"0%", "101%", "0/5", "3/2", "10GB", "500", ""} {
		_, err := Parse([]byte(minimalYAML(subset)))
		if err == nil {
			t.Fatalf("subset %s debería rechazarse", subset)
		}
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "config.yaml"))
	if err == nil || !strings.Contains(err.Error(), "no se encontró") {
		t.Fatalf("Load: %v", err)
	}
}

func TestExampleFileOnlyLacksPassword(t *testing.T) {
	_, err := Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	var verr *ValidationError
	if !asValidation(err, &verr) {
		t.Fatalf("ejemplo: %v", err)
	}
	if len(verr.Problems) != 1 || !strings.Contains(verr.Problems[0], "password_encrypted") {
		t.Fatalf("problemas del ejemplo: %#v", verr.Problems)
	}
}

func minimalYAML(subset string) string {
	return `
backup:
  sources: ["C:\\Datos"]
repository:
  path: "E:\\Copias"
  password_encrypted: "dpapi:YmxvYg=="
schedule:
  times: ["13:30"]
verify:
  read_data_subset: "` + subset + `"
`
}

func asValidation(err error, target **ValidationError) bool {
	if err == nil {
		return false
	}
	verr, ok := err.(*ValidationError)
	if !ok {
		return false
	}
	*target = verr
	return true
}
