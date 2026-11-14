package main

import (
	"strings"
	"testing"
)

func TestCancelledRestoreExplainsWhatWasLeft(t *testing.T) {
	original := cancelledRestore("original", "ubicación original", 0)
	if original.Status != "warning" || !strings.Contains(original.Message, "No se ha cambiado nada") || original.Errors == nil {
		t.Fatalf("ubicación original: %+v", original)
	}
	vacia := cancelledRestore("folder", `C:\Users\Juan\Desktop\Restaurado`, 0)
	if vacia.Status != "warning" || !strings.Contains(vacia.Message, "una parte de los archivos") || vacia.Target == "" {
		t.Fatalf("sin archivos: %+v", vacia)
	}
	parcial := cancelledRestore("folder", `C:\Users\Juan\Desktop\Restaurado`, 7)
	if parcial.Files != 7 || !strings.Contains(parcial.Message, "7 archivos") || !strings.Contains(parcial.Message, "incompleto") {
		t.Fatalf("recuperación parcial: %+v", parcial)
	}
}
