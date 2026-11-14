package main

import (
	"strings"
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
	if !ok || title != "No se ha podido hacer la copia" || body != "Destino lleno"+avisoAccion {
		t.Fatalf("fallo: %q %q %v", title, body, ok)
	}

	warned := newer
	warned.LastBackup = &status.Run{FinishedAt: "2026-11-14T20:00:00+01:00", Status: "warning"}
	if _, _, ok := backupNotice(&newer, warned, noticeOptions{}); ok {
		t.Fatal("los avisos están desactivados")
	}
	title, body, ok = backupNotice(&newer, warned, noticeOptions{warnings: true})
	if !ok || title != "La copia terminó con avisos" || !strings.HasSuffix(body, avisoAccion) {
		t.Fatalf("aviso: %q %q %v", title, body, ok)
	}

	okRun := warned
	okRun.Level = status.Green
	okRun.LastBackup = &status.Run{FinishedAt: "2026-11-15T13:30:00+01:00", Status: "success"}
	if _, _, ok := backupNotice(&warned, okRun, noticeOptions{warnings: true}); ok {
		t.Fatal("una copia correcta no avisa si no se ha pedido")
	}
	title, _, ok = backupNotice(&warned, okRun, noticeOptions{success: true})
	if !ok || title != "Copia correcta" || title == "" {
		t.Fatalf("correcta: %q %v", title, ok)
	}

	offline := okRun
	offline.Destination.Connected = false
	offline.LastBackup = &status.Run{FinishedAt: "2026-11-16T13:30:00+01:00", Status: "failed"}
	title, body, ok = backupNotice(&okRun, offline, noticeOptions{})
	if !ok || title != "No se ha podido hacer la copia" || !strings.Contains(body, "destino de las copias no está disponible") || !strings.HasSuffix(body, avisoAccion) {
		t.Fatalf("destino: %q %q %v", title, body, ok)
	}
}

func TestNoticeSaysTheUSBDiskIsNotConnected(t *testing.T) {
	before := status.View{Destination: status.Destination{Connected: true}, LastBackup: &status.Run{FinishedAt: "2026-11-13T13:30:00+01:00", Status: "success"}}
	after := before
	after.Destination = status.Destination{Connected: false, Kind: "removable"}
	after.LastBackup = &status.Run{FinishedAt: "2026-11-14T13:30:00+01:00", Status: "failed"}
	title, body, ok := backupNotice(&before, after, noticeOptions{})
	if !ok || title != "No se ha podido hacer la copia" || body != "El disco USB no está conectado. Conéctalo y vuelve a intentarlo."+avisoAccion {
		t.Fatalf("USB: %q %q %v", title, body, ok)
	}
}

func TestNoticeIgnoresADisconnectedDiskAfterASuccessfulCopy(t *testing.T) {
	before := status.View{LastBackup: &status.Run{FinishedAt: "2026-11-13T13:30:00+01:00", Status: "success"}}
	after := before
	after.Destination = status.Destination{Connected: false, Kind: "removable"} // se desconectó después de copiar bien
	after.LastBackup = &status.Run{FinishedAt: "2026-11-14T13:30:00+01:00", Status: "success"}
	if _, _, ok := backupNotice(&before, after, noticeOptions{}); ok {
		t.Fatal("una copia correcta no debe avisar de que el disco ya no está")
	}
}

func TestNoticeBodyFitsInAWindowsBalloon(t *testing.T) {
	long := strings.Repeat("palabra ", 80)
	before := status.View{LastBackup: &status.Run{FinishedAt: "2026-11-13T13:30:00+01:00", Status: "success"}, Destination: status.Destination{Connected: true}}
	after := before
	after.LastBackup = &status.Run{FinishedAt: "2026-11-14T13:30:00+01:00", Status: "failed", Message: long}
	_, body, ok := backupNotice(&before, after, noticeOptions{})
	if !ok || len([]rune(body)) > 250 || !strings.HasSuffix(body, avisoAccion) {
		t.Fatalf("el texto debe caber en el aviso y acabar con la acción: %d caracteres", len([]rune(body)))
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
