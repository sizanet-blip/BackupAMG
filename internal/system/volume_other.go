//go:build !windows

package system

import "fmt"

func VolumeGUID(string) (string, error) {
	return "", fmt.Errorf("no se pudo identificar el disco")
}

func MountOf(string) (string, error) {
	return "", fmt.Errorf("Destino no disponible: conecta el disco USB o comprueba la ruta")
}

func DriveKind(string) string { return "other" }

func VolumeLabel(string) string { return "" }

func Locate(path, volumeID string) (string, error) {
	if volumeID != "" {
		return "", fmt.Errorf("Destino no disponible: conecta el disco USB o comprueba la ruta")
	}
	return path, nil
}
