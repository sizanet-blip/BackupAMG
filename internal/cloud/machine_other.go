//go:build !windows

package cloud

import "fmt"

// MachineUID solo está implementado en Windows.
func MachineUID() (string, error) {
	return "", fmt.Errorf("la identificación del equipo solo está disponible en Windows")
}
