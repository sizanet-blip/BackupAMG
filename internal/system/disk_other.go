//go:build !windows

package system

import "fmt"

// FreeBytes no está implementado fuera de Windows.
func FreeBytes(path string) (int64, error) {
	return 0, fmt.Errorf("el espacio libre solo se consulta en Windows")
}

func DiskSpace(string) (int64, int64, error) {
	return 0, 0, fmt.Errorf("el espacio libre solo se consulta en Windows")
}
