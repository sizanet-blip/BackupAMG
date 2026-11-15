package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const replicaBase = `
agent:
  name: PRUEBA
backup:
  sources: ["C:\\Datos"]
repository:
  path: "E:\\Copias"
  password_encrypted: "dpapi:YmxvYg=="
schedule:
  times: ["13:30"]
`

func parseWith(t *testing.T, extra string) (*Config, error) {
	t.Helper()
	return Parse([]byte(replicaBase + extra))
}

func TestNoReplicasBehavesAsBefore(t *testing.T) {
	cfg, err := parseWith(t, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Replicas) != 0 {
		t.Fatalf("no debía haber copias adicionales: %+v", cfg.Replicas)
	}
}

func TestParseReplicasAppliesDefaults(t *testing.T) {
	cfg, err := parseWith(t, `
replicas:
  - id: usb-oficina
    name: Disco externo
    path: 'F:\CopiasExtra'
  - id: nas
    path: '\\nas\copias'
    when: daily_at
    time: "3:05"
    enabled: false
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Replicas) != 2 {
		t.Fatalf("%+v", cfg.Replicas)
	}
	a, b := cfg.Replicas[0], cfg.Replicas[1]
	if a.Type != ReplicaLocal || !a.Enabled || a.When != ReplicaAfterBackup || a.Name != "Disco externo" {
		t.Fatalf("valores por defecto: %+v", a)
	}
	if b.Type != ReplicaUNC || b.Enabled || b.When != ReplicaDailyAt || b.Time != "03:05" || b.Name != "nas" {
		t.Fatalf("segunda copia: %+v", b)
	}
}

func TestReplicaValidationRejectsBadPlacement(t *testing.T) {
	cases := map[string]string{
		"igual al principal":    `E:\Copias`,
		"dentro del principal":  `E:\Copias\extra`,
		"contiene al principal": `E:\`,
		"dentro de un origen":   `C:\Datos\copias`,
		"contiene a un origen":  `C:\`,
		"ruta relativa":         `copias`,
		"ruta vacía":            ``,
		"otras mayúsculas":      `e:\COPIAS`,
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseWith(t, "replicas:\n  - id: extra\n    path: '"+path+"'\n")
			if err == nil {
				t.Fatalf("%q debía rechazarse", path)
			}
		})
	}
}

func TestReplicaValidationRejectsOtherMistakes(t *testing.T) {
	cases := map[string]string{
		"id repetido":       "replicas:\n  - {id: a, path: 'F:\\x'}\n  - {id: a, path: 'G:\\y'}\n",
		"id mayúscula":      "replicas:\n  - {id: Mal, path: 'F:\\x'}\n",
		"id vacío":          "replicas:\n  - {path: 'F:\\x'}\n",
		"nube":              "replicas:\n  - {id: n, type: s3, path: 'F:\\x'}\n",
		"tipo raro":         "replicas:\n  - {id: n, type: ftp, path: 'F:\\x'}\n",
		"tipo no coincide":  "replicas:\n  - {id: n, type: unc, path: 'F:\\x'}\n",
		"diaria sin hora":   "replicas:\n  - {id: n, path: 'F:\\x', when: daily_at}\n",
		"cuándo raro":       "replicas:\n  - {id: n, path: 'F:\\x', when: siempre}\n",
		"solapadas":         "replicas:\n  - {id: a, path: 'F:\\x'}\n  - {id: b, path: 'F:\\x\\y'}\n",
		"volumen malo":      "replicas:\n  - {id: n, path: 'F:\\x', volume_id: raro}\n",
		"campo desconocido": "replicas:\n  - {id: n, path: 'F:\\x', bucket: b}\n",
	}
	for name, extra := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseWith(t, extra); err == nil {
				t.Fatal("debía rechazarse")
			}
		})
	}
	_, err := parseWith(t, "replicas:\n  - {id: n, type: s3, path: 'F:\\x'}\n")
	if err == nil || !strings.Contains(err.Error(), "nube") {
		t.Fatalf("el mensaje de la nube no es claro: %v", err)
	}
}

// R1: guardar Ajustes no puede borrar las copias adicionales.
func TestUpdateKeepsReplicas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := replicaBase + `
replicas:
  - id: usb
    name: Disco externo
    type: local
    path: 'F:\CopiasExtra'
    volume_id: '\\?\Volume{12345678-1234-1234-1234-123456789abc}\'
    enabled: true
    when: after_backup
  - id: nas
    type: unc
    path: '\\nas\copias'
    enabled: false
    when: daily_at
    time: "23:00"
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	edit := before.View()
	edit.Times = []string{"08:00", "20:00"}
	edit.KeepDaily = 10
	if err := Update(path, edit); err != nil {
		t.Fatal(err)
	}
	after, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Replicas) != 2 {
		t.Fatalf("se han perdido copias adicionales: %+v", after.Replicas)
	}
	for i := range after.Replicas {
		if after.Replicas[i] != before.Replicas[i] {
			t.Fatalf("la copia adicional %d cambió:\nantes   %+v\ndespués %+v", i, before.Replicas[i], after.Replicas[i])
		}
	}
	if after.Schedule.Times[0] != "08:00" || after.Retention.KeepDaily != 10 {
		t.Fatalf("el cambio de Ajustes no se aplicó: %+v", after.Schedule)
	}
}

func TestUpdateWithoutReplicasDoesNotAddTheSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(replicaBase), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _ := Load(path)
	if err := Update(path, cfg.View()); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "replicas") {
		t.Fatalf("no debía aparecer la sección replicas:\n%s", data)
	}
}

// El destino principal y la contraseña se conservan al tocar las copias adicionales, y al revés.
func TestSetReplicasKeepsTheRestAndSurvivesDestinationChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(replicaBase+"# nota del técnico\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	list := []Replica{{ID: "usb", Name: "Disco externo", Type: ReplicaLocal, Path: `F:\CopiasExtra`, Enabled: true, When: ReplicaAfterBackup}}
	if err := SetReplicas(path, list); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Replicas) != 1 || cfg.Replicas[0] != list[0] {
		t.Fatalf("%+v", cfg.Replicas)
	}
	if cfg.Repository.PasswordEncrypted != "dpapi:YmxvYg==" || cfg.Repository.Path != `E:\Copias` {
		t.Fatalf("se perdió el destino o la contraseña: %+v", cfg.Repository)
	}
	if err := SetDestination(path, `D:\Otros`, ""); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(path)
	if err != nil || len(cfg.Replicas) != 1 {
		t.Fatalf("cambiar el destino principal borró las copias adicionales: %+v %v", cfg, err)
	}

	before, _ := os.ReadFile(path)
	bad := []Replica{{ID: "x", Type: ReplicaLocal, Path: `D:\Otros`, Enabled: true, When: ReplicaAfterBackup}}
	if err := SetReplicas(path, bad); err == nil {
		t.Fatal("una copia adicional igual al principal no puede guardarse")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("el fichero cambió pese al error")
	}

	if err := SetReplicas(path, nil); err != nil {
		t.Fatal(err)
	}
	cfg, _ = Load(path)
	data, _ := os.ReadFile(path)
	if len(cfg.Replicas) != 0 || strings.Contains(string(data), "replicas") {
		t.Fatalf("no se quitó la sección:\n%s", data)
	}
}

func TestValidateReplicasMatchesTheFileRules(t *testing.T) {
	cfg, err := parseWith(t, "")
	if err != nil {
		t.Fatal(err)
	}
	ok := []Replica{{ID: "usb", Type: ReplicaLocal, Path: `F:\x`, Enabled: true, When: ReplicaAfterBackup}}
	if err := ValidateReplicas(cfg, ok); err != nil {
		t.Fatal(err)
	}
	bad := []Replica{{ID: "usb", Type: ReplicaLocal, Path: `E:\Copias\x`, Enabled: true, When: ReplicaAfterBackup}}
	if err := ValidateReplicas(cfg, bad); err == nil {
		t.Fatal("dentro del principal debía rechazarse")
	}
	if len(cfg.Replicas) != 0 {
		t.Fatal("la validación no debe cambiar la configuración")
	}
}
