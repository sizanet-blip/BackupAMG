//go:build !windows

package secrets

import "fmt"

// Protect cifra la contraseña. Fuera de Windows no hay DPAPI.
func Protect(password string) (string, error) {
	return "", fmt.Errorf("el cifrado DPAPI solo está disponible en Windows")
}

// Open descifra un valor producido por Protect.
func Open(encoded string) (string, error) {
	return "", fmt.Errorf("el cifrado DPAPI solo está disponible en Windows")
}
