//go:build !windows

package system

import "fmt"

// OpenURL abre una dirección con el programa predeterminado.
func OpenURL(string) error {
	return fmt.Errorf("abrir el navegador solo está disponible en Windows")
}

// FocusPanelWindow no está disponible fuera de Windows.
func FocusPanelWindow() bool {
	return false
}

// FocusBrowserForPanel no está disponible fuera de Windows.
func FocusBrowserForPanel() bool {
	return false
}

// RememberPanelWindowSoon no está disponible fuera de Windows.
func RememberPanelWindowSoon() {}
