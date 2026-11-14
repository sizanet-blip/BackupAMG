//go:build windows

package system

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// FreeBytes devuelve el espacio libre, en bytes, del volumen donde está path.
// Si path todavía no existe, usa la carpeta superior que sí exista.
func FreeBytes(path string) (int64, error) {
	current := filepath.Clean(path)
	for {
		info, err := os.Stat(current)
		if err == nil && info.IsDir() {
			return freeBytes(current)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return 0, fmt.Errorf("no se pudo consultar el espacio libre de %s", path)
		}
		current = parent
	}
}

func freeBytes(dir string) (int64, error) {
	free, _, err := diskSpace(dir)
	return free, err
}

// DiskSpace devuelve el espacio libre y el tamaño del volumen.
func DiskSpace(path string) (int64, int64, error) {
	current := filepath.Clean(path)
	for {
		info, err := os.Stat(current)
		if err == nil && info.IsDir() {
			return diskSpace(current)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return 0, 0, fmt.Errorf("no se pudo consultar el espacio libre de %s", path)
		}
		current = parent
	}
}

func diskSpace(dir string) (int64, int64, error) {
	ptr, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, 0, fmt.Errorf("no se pudo consultar el espacio libre de %s: %w", dir, err)
	}
	var available, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(ptr, &available, &total, &free); err != nil {
		return 0, 0, fmt.Errorf("no se pudo consultar el espacio libre de %s: %w", dir, err)
	}
	return int64(available), int64(total), nil
}
