//go:build !windows

package service

import "fmt"

// Running no tiene servicio que consultar fuera de Windows.
func Running() (bool, error) {
	return false, fmt.Errorf("el servicio solo está disponible en Windows")
}
