//go:build windows

package system

import "syscall"

// EnableUTF8Console hace que la consola interprete los mensajes en español.
// Sin esto, en una consola con la página de códigos tradicional «ejecución» se ve mal.
func EnableUTF8Console() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setOutput := kernel32.NewProc("SetConsoleOutputCP")
	setInput := kernel32.NewProc("SetConsoleCP")
	const cpUTF8 = 65001
	_, _, _ = setOutput.Call(uintptr(cpUTF8))
	_, _, _ = setInput.Call(uintptr(cpUTF8))
}
