package restic

import (
	"context"
	"fmt"
	"strings"
)

// fromPasswordEnv es la variable de restic con la contraseña del repositorio de origen en
// `restic copy` e `init --from-repo`. La contraseña nunca va en los argumentos.
const fromPasswordEnv = "RESTIC_FROM_PASSWORD="

// ReplicaInitRequest prepara un repositorio secundario a partir del principal.
// Se crea con los mismos parámetros de troceado (--copy-chunker-params): así lo que ya hay en los
// dos sitios se reconoce como igual y la copia adicional no ocupa el doble.
type ReplicaInitRequest struct {
	Executable string
	Repository string // destino secundario
	Source     string // repositorio principal
	Password   string // la misma contraseña para los dos
}

// InitReplica crea el repositorio secundario si todavía no existe.
func InitReplica(ctx context.Context, req ReplicaInitRequest) (InitResult, error) {
	if strings.TrimSpace(req.Password) == "" {
		return InitResult{}, fmt.Errorf("la contraseña del repositorio está vacía")
	}
	if err := locationAvailable(req.Repository); err != nil {
		return InitResult{}, err
	}
	if repositoryInitialized(req.Repository) {
		return InitResult{AlreadyExists: true, Message: "El repositorio ya existe"}, nil
	}
	if !repositoryInitialized(req.Source) {
		return InitResult{}, fmt.Errorf("la copia principal todavía no está preparada")
	}
	var parsed []Line
	args := []string{"init", "--json", "--from-repo", req.Source, "--copy-chunker-params"}
	code, _, stderr, runErr := runWithEnv(ctx, req.Executable, req.Password, req.Repository,
		[]string{fromPasswordEnv + req.Password}, args, func(line Line) { parsed = append(parsed, line) })
	if runErr != nil && code == 0 {
		return InitResult{}, runErr
	}
	if code == 0 {
		id := ""
		for _, line := range parsed {
			if line.Initialized != nil {
				id = line.Initialized.ID
			}
		}
		return InitResult{ID: id, Message: "Repositorio inicializado"}, nil
	}
	if repositoryInitialized(req.Repository) {
		return InitResult{AlreadyExists: true, Message: "El repositorio ya existe"}, nil
	}
	return InitResult{}, policyError("no se pudo preparar la copia adicional", code, redact(stderr, req.Password))
}

// CopyRequest replica las instantáneas que faltan del repositorio principal en el secundario.
type CopyRequest struct {
	Executable  string
	Source      string
	Destination string
	Password    string
}

// CopyResult es lo que dejó `restic copy`.
type CopyResult struct {
	// Copied es el número de instantáneas nuevas en el secundario. Cero si ya estaba al día.
	Copied    int
	Cancelled bool
}

// Copy ejecuta `restic copy`. Si el contexto se cancela (por ejemplo porque llega la copia
// principal), devuelve Cancelled sin error: lo ya copiado se conserva y la próxima vez continúa.
func Copy(ctx context.Context, req CopyRequest) (CopyResult, error) {
	if strings.TrimSpace(req.Password) == "" {
		return CopyResult{}, fmt.Errorf("la contraseña del repositorio está vacía")
	}
	if err := locationAvailable(req.Destination); err != nil {
		return CopyResult{}, err
	}
	if !repositoryInitialized(req.Source) {
		return CopyResult{}, fmt.Errorf("la copia principal todavía no está preparada")
	}
	if !repositoryInitialized(req.Destination) {
		return CopyResult{}, fmt.Errorf("la copia adicional todavía no está preparada")
	}
	code, stdout, stderr, runErr := runWithEnv(ctx, req.Executable, req.Password, req.Destination,
		[]string{fromPasswordEnv + req.Password}, []string{"copy", "--from-repo", req.Source}, nil)
	if runErr != nil && ctx.Err() != nil {
		return CopyResult{Copied: countCopied(stdout), Cancelled: true}, nil
	}
	if runErr != nil && code == 0 {
		return CopyResult{}, runErr
	}
	if code == 12 {
		// ¿Es la copia adicional la que tiene otra contraseña? Se distingue de la principal.
		if ok, exists, _ := PasswordOK(ctx, req.Executable, req.Destination, req.Password); exists && !ok {
			return CopyResult{}, ErrReplicaOtherPassword
		}
	}
	if code != 0 {
		return CopyResult{Copied: countCopied(stdout)}, policyError("no se pudo actualizar la copia adicional", code, redact(stderr, req.Password))
	}
	return CopyResult{Copied: countCopied(stdout)}, nil
}

// countCopied cuenta las líneas «snapshot X saved, copied from source snapshot Y».
func countCopied(stdout string) int {
	n := 0
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, " saved, copied from source snapshot ") {
			n++
		}
	}
	return n
}
