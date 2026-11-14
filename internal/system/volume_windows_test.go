//go:build windows

package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVolumeGUIDRoundTrip(t *testing.T) {
	id, err := VolumeGUID(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(id), `volume{`) {
		t.Fatalf("identificador: %s", id)
	}
	mount, err := MountOf(id)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := Locate(`Z:\Copias`, id)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(mount, "Copias")
	if !strings.EqualFold(resolved, want) {
		t.Fatalf("resuelto %s, se esperaba %s", resolved, want)
	}
	kind := DriveKind(mount)
	if kind != "fixed" && kind != "removable" {
		t.Fatalf("tipo de disco: %s", kind)
	}
	if DriveKind(`\\servidor\copias`) != "unc" {
		t.Fatal("una ruta UNC no se reconoció")
	}
}
