package report

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"
)

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewUIDIsAUniqueUUIDv4(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		uid := NewUID()
		if !uuidV4.MatchString(uid) {
			t.Fatalf("no es un UUID v4: %s", uid)
		}
		if seen[uid] {
			t.Fatalf("repetido: %s", uid)
		}
		seen[uid] = true
	}
}

// Cada informe lleva su report_uid, y el valor se conserva al guardarlo y leerlo.
func TestEveryReportGetsAStableReportUID(t *testing.T) {
	now := time.Now()
	rep := Build(Input{Started: now, Finished: now, Status: "success"})
	if !uuidV4.MatchString(rep.ReportUID) {
		t.Fatalf("falta el report_uid: %q", rep.ReportUID)
	}
	dir := t.TempDir()
	path, err := Write(dir, rep, now)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.TrimSuffix(path[strings.LastIndex(path, "\\")+1:], ".json")
	back, err := Open(dir, id)
	if err != nil || back.ReportUID != rep.ReportUID {
		t.Fatalf("el report_uid no se conservó: %+v %v", back, err)
	}
	// Si se indica uno, se respeta.
	fixed := Build(Input{Started: now, Finished: now, Status: "success", ReportUID: "mi-uid-0001"})
	if fixed.ReportUID != "mi-uid-0001" {
		t.Fatal("no respeta el indicado")
	}
}

// Un informe antiguo (sin report_uid) se lee igual, y no gana el campo al volver a guardarse sin él.
func TestOldReportsWithoutReportUIDStillParse(t *testing.T) {
	var old Report
	if err := json.Unmarshal([]byte(`{"schema_version":1,"type":"backup","status":"success","started_at":"2026-11-13T13:30:00+01:00","finished_at":"2026-11-13T13:34:12+01:00"}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.ReportUID != "" {
		t.Fatal("no debía tener report_uid")
	}
	data, _ := json.Marshal(old)
	if strings.Contains(string(data), "report_uid") {
		t.Fatalf("un informe sin uid no debe escribir el campo: %s", data)
	}
}

func TestLegacyUIDIsStableAndDependsOnMachineAndFile(t *testing.T) {
	a := LegacyUID("m1", "2026-11-13_13-30-00.000.json")
	if a != LegacyUID("m1", "2026-11-13_13-30-00.000.json") {
		t.Fatal("no es estable")
	}
	if a == LegacyUID("m2", "2026-11-13_13-30-00.000.json") || a == LegacyUID("m1", "otro.json") {
		t.Fatal("debe depender del equipo y del fichero")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`).MatchString(a) {
		t.Fatalf("formato no válido para el servidor: %s", a)
	}
}
