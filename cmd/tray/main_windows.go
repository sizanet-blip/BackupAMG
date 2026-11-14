//go:build windows

package main

import (
	"os"
	"sync/atomic"
	"time"

	"backup-agent/internal/panel"
	"backup-agent/internal/system"
	"golang.org/x/sys/windows"
)

var openPanelBusy atomic.Bool

func main() {
	if alreadyRunning() {
		os.Exit(0)
	}
	app := &trayApp{}
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if app.hwnd != 0 {
				pPostMessage.Call(uintptr(app.hwnd), wmAppRefresh, 0, 0)
			}
		}
	}()
	app.run()
}

func alreadyRunning() bool {
	name, err := windows.UTF16PtrFromString(`Local\BackupAgentTray`)
	if err != nil {
		return false
	}
	_, err = windows.CreateMutex(nil, false, name)
	return err == windows.ERROR_ALREADY_EXISTS
}

func openPanel(next string) {
	if !openPanelBusy.CompareAndSwap(false, true) {
		pushToast("Un momento", "Ya se está abriendo el panel.")
		return
	}
	defer openPanelBusy.Store(false)

	// Navegar por SSE si la ventana-app del panel sigue viva.
	_, url, _, _, err := panel.RequestShow(next)
	if system.FocusPanelWindow() {
		return
	}
	if err != nil {
		pushToast("No se pudo abrir el panel", "El agente no está en marcha.")
		return
	}
	// Sin ventana a la vista: abrir (o reactivar) la ventana-app dedicada.
	if url == "" {
		tokenURL, _, _, tokenErr := panel.RequestToken()
		if tokenErr != nil {
			pushToast("No se pudo abrir el panel", "El agente no está en marcha.")
			return
		}
		url = tokenURL
		if next == "historial" || next == "restaurar" {
			url += "&next=" + next
		}
	}
	if err := system.OpenURL(url); err != nil {
		pushToast("No se pudo abrir el panel", err.Error())
	}
}