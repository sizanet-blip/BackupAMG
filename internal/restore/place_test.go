package restore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathConversion(t *testing.T) {
	win := `C:\Datos\informe.txt`
	got := ToWindows("/C/Datos/informe.txt")
	if got != win {
		t.Fatalf("a Windows: %s", got)
	}
	if back := ToRestic(win); back != "/C/Datos/informe.txt" {
		t.Fatalf("a restic: %s", back)
	}
	if ToWindows(win) != win {
		t.Fatal("una ruta de Windows no debe cambiar")
	}
	if ToWindows(`C\Datos\informe.txt`) != win {
		t.Fatal("una ruta extraída no volvió a su sitio")
	}
}

func TestPlaceFileKeepsTheExistingOne(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "informe.txt")
	if err := os.WriteFile(dest, []byte("viejo"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "nuevo.txt")
	if err := os.WriteFile(src, []byte("nuevo"), 0o644); err != nil {
		t.Fatal(err)
	}
	final, renamed, err := placeFile(src, dest, false)
	if err != nil || !renamed {
		t.Fatalf("renombrar: %s %v %v", final, renamed, err)
	}
	old, err := os.ReadFile(dest)
	if err != nil || string(old) != "viejo" {
		t.Fatalf("el original cambió: %s %v", old, err)
	}
	got, err := os.ReadFile(final)
	if err != nil || string(got) != "nuevo" {
		t.Fatalf("recuperado: %s %v", got, err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("el temporal sigue en su sitio")
	}
}

func TestPlaceFileCanOverwrite(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "informe.txt")
	if err := os.WriteFile(dest, []byte("viejo"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "nuevo.txt")
	if err := os.WriteFile(src, []byte("nuevo"), 0o644); err != nil {
		t.Fatal(err)
	}
	final, renamed, err := placeFile(src, dest, true)
	if err != nil || renamed || final != dest {
		t.Fatalf("sustituir: %s %v %v", final, renamed, err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != "nuevo" {
		t.Fatalf("contenido: %s %v", got, err)
	}
}
