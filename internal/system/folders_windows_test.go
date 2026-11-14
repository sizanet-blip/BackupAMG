//go:build windows

package system

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAddDriveInfoSkipsSlowDrive(t *testing.T) {
	dirs := []Dir{{Name: `C:\`, Path: `C:\`}, {Name: `Z:\`, Path: `Z:\`}}
	release := make(chan struct{})
	defer close(release)
	probe := func(path string) (driveData, bool) {
		if path == `Z:\` {
			<-release
			return driveData{}, false
		}
		return driveData{label: "Sistema", free: 100, total: 400}, true
	}
	start := time.Now()
	addDriveInfo(dirs, 200*time.Millisecond, probe)
	if time.Since(start) > 2*time.Second {
		t.Fatal("la lista se ha bloqueado esperando a una unidad")
	}
	if dirs[0].Label != "Sistema" || dirs[0].FreeBytes != 100 || dirs[0].TotalBytes != 400 {
		t.Fatalf("faltan datos de la unidad que responde: %+v", dirs[0])
	}
	if dirs[1].Label != "" || dirs[1].FreeBytes != 0 {
		t.Fatalf("la unidad lenta no debería llevar datos: %+v", dirs[1])
	}
}

func TestDirJSONKeepsOldFieldsAndOmitsEmpty(t *testing.T) {
	raw, err := json.Marshal(Dir{Name: `D:\`, Path: `D:\`})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"name":"D:\\","path":"D:\\"}` {
		t.Fatalf("sin datos solo deben salir name y path: %s", raw)
	}
	raw, _ = json.Marshal(Dir{Name: `D:\`, Path: `D:\`, Label: "Copias", FreeBytes: 5, TotalBytes: 10})
	for _, parte := range []string{`"name"`, `"path"`, `"label":"Copias"`, `"free_bytes":5`, `"total_bytes":10`} {
		if !strings.Contains(string(raw), parte) {
			t.Fatalf("falta %s en %s", parte, raw)
		}
	}
}

func TestDrivesReturnsData(t *testing.T) {
	list := drives()
	if len(list) == 0 {
		t.Skip("sin unidades")
	}
	for _, d := range list {
		if d.Path == `C:\` && d.TotalBytes <= 0 {
			t.Fatalf("C: debería traer su tamaño: %+v", d)
		}
	}
}
