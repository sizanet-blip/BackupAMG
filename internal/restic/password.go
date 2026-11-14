package restic

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PasswordOK comprueba si la contraseña abre un repositorio que ya existe.
// El segundo valor es falso cuando la carpeta todavía no es un repositorio.
func PasswordOK(ctx context.Context, exe, repo, password string) (bool, bool, error) {
	if !repositoryInitialized(repo) {
		return false, false, nil
	}
	code, _, stderr, err := run(ctx, exe, password, repo, []string{"cat", "config"}, nil)
	if err != nil && code == 0 {
		return false, true, err
	}
	if code == 0 {
		return true, true, nil
	}
	if code == 12 {
		return false, true, nil
	}
	return false, true, fmt.Errorf("no se pudo comprobar el destino: %s", redact(stderr, password))
}

// ChangePassword cambia la contraseña de un repositorio ya creado.
// La contraseña nueva va en un fichero temporal, no en la lista de argumentos.
func ChangePassword(ctx context.Context, exe, repo, oldPassword, newPassword string) error {
	dir, err := os.MkdirTemp("", "ba-pw-")
	if err != nil {
		return fmt.Errorf("no se pudo preparar el cambio de contraseña")
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "nueva")
	if err := os.WriteFile(file, []byte(newPassword), 0o600); err != nil {
		return fmt.Errorf("no se pudo preparar el cambio de contraseña")
	}
	code, _, stderr, runErr := run(ctx, exe, oldPassword, repo, []string{"key", "passwd", "--new-password-file", file}, nil)
	if runErr != nil && code == 0 {
		return fmt.Errorf("no se pudo cambiar la contraseña")
	}
	if code != 0 {
		detail := redact(stderr, oldPassword, newPassword)
		if detail == "" {
			return fmt.Errorf("no se pudo cambiar la contraseña")
		}
		return fmt.Errorf("no se pudo cambiar la contraseña: %s", detail)
	}
	return nil
}

func redact(text string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "")
		}
	}
	text = strings.TrimSpace(text)
	if len(text) > 300 {
		text = text[:300]
	}
	return text
}
