//go:build windows

package main

// pushToast muestra un aviso del icono de la bandeja (Shell_NotifyIcon).
// Se encola al hilo de UI: Shell_NotifyIcon desde otros hilos colgaba el icono.

func pushToast(title, body string) {
	if activeTray == nil {
		return
	}
	activeTray.mu.Lock()
	activeTray.toastTitle = title
	activeTray.toastBody = body
	activeTray.toastReady = true
	hwnd := activeTray.hwnd
	activeTray.mu.Unlock()
	if hwnd != 0 {
		pPostMessage.Call(uintptr(hwnd), wmAppToast, 0, 0)
	}
}
