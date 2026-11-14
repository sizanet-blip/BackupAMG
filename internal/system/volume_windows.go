//go:build windows

package system

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

const (
	driveRemovable = 2
	driveFixed     = 3
	driveRemote    = 4
)

// VolumeGUID devuelve el identificador del disco donde está path.
func VolumeGUID(path string) (string, error) {
	dir, err := existingDir(path)
	if err != nil {
		return "", err
	}
	root := driveRoot(dir)
	if root == "" {
		return "", fmt.Errorf("no se pudo identificar el disco")
	}
	ptr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return "", fmt.Errorf("no se pudo identificar el disco: %w", err)
	}
	buf := make([]uint16, 100)
	if err := windows.GetVolumeNameForVolumeMountPoint(ptr, &buf[0], uint32(len(buf))); err != nil {
		return "", fmt.Errorf("no se pudo identificar el disco: %w", err)
	}
	return windows.UTF16ToString(buf), nil
}

// MountOf devuelve la letra actual de un disco, por ejemplo F:\.
func MountOf(volumeID string) (string, error) {
	volumeID = strings.TrimSpace(volumeID)
	if volumeID != "" && !strings.HasSuffix(volumeID, `\`) {
		volumeID += `\`
	}
	ptr, err := windows.UTF16PtrFromString(volumeID)
	if err != nil {
		return "", fmt.Errorf("Destino no disponible: conecta el disco USB o comprueba la ruta")
	}
	buf := make([]uint16, 512)
	var n uint32
	err = windows.GetVolumePathNamesForVolumeName(ptr, &buf[0], uint32(len(buf)), &n)
	if err == windows.ERROR_MORE_DATA && n > 0 {
		buf = make([]uint16, n)
		err = windows.GetVolumePathNamesForVolumeName(ptr, &buf[0], uint32(len(buf)), &n)
	}
	if err != nil {
		return "", fmt.Errorf("Destino no disponible: conecta el disco USB o comprueba la ruta")
	}
	var letter string
	for _, mount := range splitUTF16(buf) {
		if len(mount) == 3 && mount[1] == ':' {
			letter = mount
			break
		}
		if letter == "" {
			letter = mount
		}
	}
	if letter == "" {
		return "", fmt.Errorf("Destino no disponible: conecta el disco USB o comprueba la ruta")
	}
	if !strings.HasSuffix(letter, `\`) {
		letter += `\`
	}
	return letter, nil
}

// DriveKind dice si la ruta es un disco fijo, extraíble, una unidad de red o no está.
// VolumeLabel es el nombre que Windows muestra del disco, por ejemplo «Copias».
func VolumeLabel(path string) string {
	root := driveRoot(path)
	if root == "" {
		return ""
	}
	ptr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return ""
	}
	var label [260]uint16
	if err := windows.GetVolumeInformation(ptr, &label[0], uint32(len(label)), nil, nil, nil, nil, 0); err != nil {
		return ""
	}
	return strings.TrimSpace(windows.UTF16ToString(label[:]))
}

func DriveKind(path string) string {
	slash := strings.ReplaceAll(path, "/", `\`)
	if strings.HasPrefix(slash, `\\`) && !strings.HasPrefix(strings.ToLower(slash), `\\?\`) {
		return "unc"
	}
	root := driveRoot(path)
	if root == "" {
		return "other"
	}
	ptr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return "other"
	}
	switch windows.GetDriveType(ptr) {
	case driveRemovable:
		return "removable"
	case driveFixed:
		return "fixed"
	case driveRemote:
		return "remote"
	default:
		return "missing"
	}
}

// Locate aplica el identificador de disco. Sin identificador devuelve path tal cual.
func Locate(path, volumeID string) (string, error) {
	if strings.TrimSpace(volumeID) == "" {
		return path, nil
	}
	mount, err := MountOf(volumeID)
	if err != nil {
		return "", err
	}
	return JoinRepo(path, mount), nil
}

func existingDir(path string) (string, error) {
	current := filepath.Clean(path)
	for {
		info, err := os.Stat(current)
		if err == nil && info.IsDir() {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no se encuentra el disco")
		}
		current = parent
	}
}

func driveRoot(path string) string {
	path = filepath.Clean(path)
	if len(path) >= 2 && path[1] == ':' {
		return strings.ToUpper(path[:1]) + `:\`
	}
	return ""
}

func splitUTF16(buf []uint16) []string {
	var out []string
	start := 0
	for i, char := range buf {
		if char != 0 {
			continue
		}
		if i == start {
			break
		}
		out = append(out, windows.UTF16ToString(buf[start:i]))
		start = i + 1
	}
	return out
}
