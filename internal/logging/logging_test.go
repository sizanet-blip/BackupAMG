package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNewWritesReadableLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.log")

	var console bytes.Buffer
	prev := osStdout
	osStdout = &console
	t.Cleanup(func() { osStdout = prev })

	log, err := New(Options{LogFile: path, Console: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	log.Info("Agente iniciado", "version", "0.1.0", "mode", "console")
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}

	fileBody, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range [][]byte{fileBody, console.Bytes()} {
		if !bytes.Contains(body, []byte("Agente iniciado")) {
			t.Fatalf("falta el mensaje: %q", body)
		}
		if !bytes.Contains(body, []byte("version=0.1.0")) {
			t.Fatalf("falta la versión: %q", body)
		}
	}
}
