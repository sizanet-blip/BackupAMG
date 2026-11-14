// Package secrets cifra y descifra la contraseña del repositorio.
// En disco solo se guarda el resultado, con el prefijo "dpapi:".
package secrets

import (
	"encoding/base64"
	"strings"
)

const prefix = "dpapi:"

// Valid comprueba que el valor guardado tiene forma de contraseña cifrada.
// No descifra: sirve para rechazar una contraseña escrita en claro.
func Valid(encoded string) bool {
	rest, ok := strings.CutPrefix(strings.TrimSpace(encoded), prefix)
	if !ok || rest == "" {
		return false
	}
	raw, err := base64.StdEncoding.DecodeString(rest)
	return err == nil && len(raw) > 0
}

func encode(ciphertext []byte) string {
	return prefix + base64.StdEncoding.EncodeToString(ciphertext)
}

func decode(encoded string) ([]byte, error) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(encoded), prefix)
	if !ok {
		return nil, errNotEncrypted
	}
	raw, err := base64.StdEncoding.DecodeString(rest)
	if err != nil || len(raw) == 0 {
		return nil, errNotEncrypted
	}
	return raw, nil
}
