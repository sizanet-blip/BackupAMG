package restic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// KeyInfo es una clave (una contraseña) de un repositorio. Un repositorio puede tener varias.
type KeyInfo struct {
	ID string
	// Current es la clave con la que se ha abierto el repositorio en esa consulta.
	Current bool
}

var newKeyID = regexp.MustCompile(`with ID ([0-9a-f]{64})`)

// ErrWrongPassword es que la contraseña no abre el repositorio.
var ErrWrongPassword = errors.New("la contraseña del repositorio no es correcta")

// ErrReplicaOtherPassword es que la copia adicional no se abre con la contraseña de la principal.
var ErrReplicaOtherPassword = errors.New("Esta copia adicional tiene otra contraseña que la copia principal.")

// KeyList devuelve las claves del repositorio.
func KeyList(ctx context.Context, exe, repo, password string) ([]KeyInfo, error) {
	code, stdout, stderr, err := run(ctx, exe, password, repo, []string{"key", "list", "--json"}, nil)
	if err != nil && code == 0 {
		return nil, err
	}
	if code == 12 {
		return nil, ErrWrongPassword
	}
	if code != 0 {
		return nil, policyError("no se pudo leer las claves", code, redact(stderr, password))
	}
	var raw []struct {
		ID      string `json:"id"`
		Current bool   `json:"current"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &raw); err != nil {
		return nil, fmt.Errorf("no se pudo leer las claves del repositorio")
	}
	out := make([]KeyInfo, 0, len(raw))
	for _, k := range raw {
		out = append(out, KeyInfo{ID: k.ID, Current: k.Current})
	}
	return out, nil
}

// CurrentKeyID es el identificador de la clave con la que se abre el repositorio.
func CurrentKeyID(ctx context.Context, exe, repo, password string) (string, error) {
	keys, err := KeyList(ctx, exe, repo, password)
	if err != nil {
		return "", err
	}
	for _, k := range keys {
		if k.Current {
			return k.ID, nil
		}
	}
	return "", fmt.Errorf("no se encontró la clave actual del repositorio")
}

// KeyAdd añade una contraseña nueva SIN quitar la actual: el repositorio acepta las dos.
// Devuelve el identificador de la clave nueva. La contraseña nueva va en un fichero temporal.
func KeyAdd(ctx context.Context, exe, repo, password, newPassword string) (string, error) {
	if strings.TrimSpace(newPassword) == "" {
		return "", fmt.Errorf("la contraseña nueva está vacía")
	}
	dir, err := os.MkdirTemp("", "ba-pw-")
	if err != nil {
		return "", fmt.Errorf("no se pudo preparar el cambio de contraseña")
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "nueva")
	if err := os.WriteFile(file, []byte(newPassword), 0o600); err != nil {
		return "", fmt.Errorf("no se pudo preparar el cambio de contraseña")
	}
	code, stdout, stderr, runErr := run(ctx, exe, password, repo, []string{"key", "add", "--new-password-file", file}, nil)
	if runErr != nil && code == 0 {
		return "", fmt.Errorf("no se pudo añadir la contraseña nueva")
	}
	if code == 12 {
		return "", ErrWrongPassword
	}
	if code != 0 {
		return "", policyError("no se pudo añadir la contraseña nueva", code, redact(stderr, password, newPassword))
	}
	if m := newKeyID.FindStringSubmatch(stdout + "\n" + stderr); m != nil {
		return m[1], nil
	}
	return "", fmt.Errorf("se añadió la contraseña nueva pero no se pudo identificar su clave")
}

// KeyRemove quita una clave. No se puede quitar la clave con la que se está abriendo el repositorio.
func KeyRemove(ctx context.Context, exe, repo, password, id string) error {
	if !regexp.MustCompile(`^[0-9a-f]{8,64}$`).MatchString(id) {
		return fmt.Errorf("identificador de clave no válido")
	}
	code, _, stderr, runErr := run(ctx, exe, password, repo, []string{"key", "remove", id}, nil)
	if runErr != nil && code == 0 {
		return fmt.Errorf("no se pudo quitar la contraseña antigua")
	}
	if code == 12 {
		return ErrWrongPassword
	}
	if code != 0 {
		return policyError("no se pudo quitar la contraseña antigua", code, redact(stderr, password))
	}
	return nil
}

// HasKey dice si el repositorio tiene esa clave.
func HasKey(ctx context.Context, exe, repo, password, id string) (bool, error) {
	keys, err := KeyList(ctx, exe, repo, password)
	if err != nil {
		return false, err
	}
	for _, k := range keys {
		if k.ID == id {
			return true, nil
		}
	}
	return false, nil
}
