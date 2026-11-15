//go:build windows

package system

import (
	"syscall"

	"golang.org/x/sys/windows"
)

const (
	esContinuous     = 0x80000000
	esSystemRequired = 0x00000001
)

var setThreadExecutionState = syscall.NewLazyDLL("kernel32.dll").NewProc("SetThreadExecutionState")

// PreventSleep impide que Windows suspenda el equipo mientras dura la copia.
// Hay que llamar a la función devuelta al terminar.
func PreventSleep() func() {
	_, _, _ = setThreadExecutionState.Call(uintptr(esContinuous | esSystemRequired))
	return func() {
		_, _, _ = setThreadExecutionState.Call(uintptr(esContinuous))
	}
}

// PIDAlive indica si el proceso sigue en ejecución.
// Si Windows niega la consulta, se considera que sigue vivo para no borrar un bloqueo ajeno.
func PIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if err == nil {
		defer windows.CloseHandle(handle)
		// Un proceso terminado sigue siendo "abrible" mientras alguien tenga un
		// manejador suyo (antivirus, el padre...). Hay que mirar si ya ha acabado.
		event, waitErr := windows.WaitForSingleObject(handle, 0)
		if waitErr != nil {
			return true
		}
		return event == uint32(windows.WAIT_TIMEOUT)
	}
	return err == windows.ERROR_ACCESS_DENIED
}
