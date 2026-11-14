package main

import (
	"testing"
	"time"

	"backup-agent/internal/status"
)

func TestBackupNoticeOnlyOnANewResult(t *testing.T) {
	current := status.View{
		Level:       status.Red,
		Destination: status.Destination{Connected: true},
		LastBackup:  &status.Run{FinishedAt: "2026-11-13T23:06:31+01:00", Status: "failed", Message: "Destino lleno"},
	}
	if _, _, ok := backupNotice(nil, current, noticeOptions{warnings: true}); ok {
		t.Fatal("el primer vistazo no debe avisar de una copia antigua")
	}
	same := current
	if _, _, ok := backupNotice(&same, current, noticeOptions{warnings: true}); ok {
		t.Fatal("repetir el mismo resultado no debe avisar")
	}

	newer := current
	newer.LastBackup = &status.Run{FinishedAt: "2026-11-14T13:30:00+01:00", Status: "failed", Message: "Destino lleno"}
	title, body, ok := backupNotice(&current, newer, noticeOptions{warnings: true})
	if !ok || title != "La copia ha fallado" || body != "Destino lleno" {
		t.Fatalf("fallo: %q %q %v", title, body, ok)
	}

	warned := newer
	warned.LastBackup = &status.Run{FinishedAt: "2026-11-14T20:00:00+01:00", Status: "warning"}
	if _, _, ok := backupNotice(&newer, warned, noticeOptions{}); ok {
		t.Fatal("los avisos están desactivados")
	}
	title, _, ok = backupNotice(&newer, warned, noticeOptions{warnings: true})
	if !ok || title != "La copia terminó con avisos" {
		t.Fatalf("aviso: %q %v", title, ok)
	}

	okRun := warned
	okRun.Level = status.Green
	okRun.LastBackup = &status.Run{FinishedAt: "2026-11-15T13:30:00+01:00", Status: "success"}
	if _, _, ok := backupNotice(&warned, okRun, noticeOptions{warnings: true}); ok {
		t.Fatal("una copia correcta no avisa si no se ha pedido")
	}
	title, _, ok = backupNotice(&warned, okRun, noticeOptions{success: true})
	if !ok || title != "Copia correcta" {
		t.Fatalf("correcta: %q %v", title, ok)
	}

	offline := okRun
	offline.Destination.Connected = false
	offline.LastBackup = &status.Run{FinishedAt: "2026-11-16T13:30:00+01:00", Status: "failed"}
	title, _, ok = backupNotice(&okRun, offline, noticeOptions{})
	if !ok || title != "Destino no conectado" {
		t.Fatalf("destino: %q %v", title, ok)
	}
}

func TestMenuLinesUsePlainSpanish(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.Local)
	title, detail := menuLines(status.View{}, false, now)
	if title != "Servicio parado" || detail != "Las copias no se están haciendo" {
		t.Fatal(title, detail)
	}
	ultima, _ := time.Parse(time.RFC3339, "2026-11-13T23:06:31+01:00")
	proxima, _ := time.Parse(time.RFC3339, "2026-11-14T20:00:00+01:00")
	title, detail = menuLines(status.View{
		Level:      status.Green,
		LastBackup: &status.Run{FinishedAt: ultima.Format(time.RFC3339), Status: "success"},
		NextBackup: proxima.Format(time.RFC3339),
	}, true, now)
	if title != "Copias al día" {
		t.Fatal(title)
	}
	if detail != "Última: ayer "+ultima.In(time.Local).Format("15:04")+" · Próxima: hoy "+proxima.In(time.Local).Format("15:04") {
		t.Fatal(detail)
	}
}
