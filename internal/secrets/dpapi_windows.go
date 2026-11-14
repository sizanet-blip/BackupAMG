//go:build windows

package secrets

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Ámbito de máquina: el servicio, que corre como LocalSystem, puede descifrarla.
// CRYPTPROTECT_UI_FORBIDDEN evita cualquier ventana si el descifrado falla.
const protectFlags = windows.CRYPTPROTECT_LOCAL_MACHINE | windows.CRYPTPROTECT_UI_FORBIDDEN

// Protect cifra la contraseña con DPAPI y devuelve el valor que se guarda en el YAML.
func Protect(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("la contraseña está vacía")
	}
	plain := []byte(password)
	in := windows.DataBlob{Size: uint32(len(plain)), Data: &plain[0]}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, protectFlags, &out); err != nil {
		return "", fmt.Errorf("no se pudo cifrar la contraseña: %w", err)
	}
	defer freeBlob(out)
	return encode(copyBlob(out)), nil
}

// Open descifra un valor producido por Protect.
func Open(encoded string) (string, error) {
	raw, err := decode(encoded)
	if err != nil {
		return "", err
	}
	in := windows.DataBlob{Size: uint32(len(raw)), Data: &raw[0]}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, protectFlags, &out); err != nil {
		return "", fmt.Errorf("no se pudo descifrar la contraseña. Vuelve a ejecutar agent set-password en este equipo: %w", err)
	}
	defer freeBlob(out)
	return string(copyBlob(out)), nil
}

func copyBlob(b windows.DataBlob) []byte {
	if b.Data == nil || b.Size == 0 {
		return nil
	}
	return append([]byte(nil), unsafe.Slice(b.Data, b.Size)...)
}

func freeBlob(b windows.DataBlob) {
	if b.Data != nil {
		_, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(b.Data)))
	}
}
