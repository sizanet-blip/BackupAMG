package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRotateKeepsLimitedBackups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.log")
	w, err := NewRotatingFile(path, 40, 2)
	if err != nil {
		t.Fatalf("NewRotatingFile: %v", err)
	}

	line := []byte("0123456789abcdef\n") // 17 bytes
	for i := 0; i < 12; i++ {
		if _, err := w.Write(line); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for _, name := range []string{"agent.log", "agent.log.1", "agent.log.2"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("se esperaba %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "agent.log.3")); !os.IsNotExist(err) {
		t.Fatal("no debe existir agent.log.3")
	}
}

func TestRotatePreservesExistingContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.log")
	if err := os.WriteFile(path, []byte("previo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := NewRotatingFile(path, 1024, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("nuevo\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("previo\n")) || !bytes.Contains(got, []byte("nuevo\n")) {
		t.Fatalf("contenido inesperado: %q", got)
	}
}

func TestNewRotatingFileRejectsBadLimits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.log")
	if _, err := NewRotatingFile(path, 0, 1); err == nil {
		t.Fatal("se esperaba error con tamaño cero")
	}
	if _, err := NewRotatingFile(path, 10, 0); err == nil {
		t.Fatal("se esperaba error sin copias")
	}
}
