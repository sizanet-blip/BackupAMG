// Package restic ejecuta restic.exe y traduce su salida JSON.
// El agente no reimplementa el motor de copia.
package restic

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// StatusSuccess es una copia terminada sin ficheros omitidos.
	StatusSuccess = "success"
	// StatusWarning es una copia creada, con algún fichero que no se pudo leer.
	StatusWarning = "warning"
	// StatusFailed es una copia que no dejó un snapshot válido.
	StatusFailed = "failed"
)

// Executable es restic.exe situado junto al ejecutable del agente.
func Executable() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("no se pudo localizar el agente: %w", err)
	}
	path := filepath.Join(filepath.Dir(self), "restic.exe")
	path, err = filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("no se pudo localizar restic.exe: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("no se encontró restic.exe junto al agente (%s)", path)
	}
	return path, nil
}

// Version ejecuta `restic version` y devuelve el número, por ejemplo "0.19.1".
func Version(ctx context.Context, exe string) (string, error) {
	cmd := command(ctx, exe, "version")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("no se pudo consultar la versión de restic: %w", err)
	}
	fields := strings.Fields(string(out))
	if len(fields) < 2 || fields[0] != "restic" {
		return "", fmt.Errorf("respuesta de versión no reconocida: %s", strings.TrimSpace(string(out)))
	}
	return fields[1], nil
}

// BackupRequest es una copia inmediata. La contraseña viaja en el entorno del
// proceso hijo, nunca en la lista de argumentos.
type BackupRequest struct {
	Executable string
	Repository string
	Password   string
	Sources    []string
	Excludes   []string
	UseVSS     bool
	Host       string
	OnStatus   func(Status)
}

// SkippedFile es un fichero que restic no pudo leer.
type SkippedFile struct {
	Path   string
	Reason string
}

// BackupOutcome es el resultado ya interpretado de `restic backup`.
type BackupOutcome struct {
	Status              string
	ExitCode            int
	Message             string
	SnapshotID          string
	FilesNew            int
	FilesChanged        int
	FilesUnmodified     int
	DataAdded           int64
	TotalFilesProcessed int
	TotalBytesProcessed int64
	DurationSeconds     float64
	Skipped             []SkippedFile
	ResticVersion       string
}

// Text es el resumen legible de la copia.
func (o BackupOutcome) Text() string {
	var b strings.Builder
	b.WriteString(o.Message)
	if o.SnapshotID != "" {
		fmt.Fprintf(&b, "\nInstantánea: %s", o.SnapshotID)
	}
	if o.Status != StatusFailed {
		fmt.Fprintf(&b, "\nFicheros nuevos: %d, modificados: %d, sin cambios: %d", o.FilesNew, o.FilesChanged, o.FilesUnmodified)
		fmt.Fprintf(&b, "\nDatos añadidos: %d bytes", o.DataAdded)
	}
	for _, file := range o.Skipped {
		fmt.Fprintf(&b, "\nNo se pudo leer %s: %s", file.Path, file.Reason)
	}
	return b.String()
}

// Backup comprueba el destino, ejecuta restic y traduce el código de salida.
func Backup(ctx context.Context, req BackupRequest) (BackupOutcome, error) {
	if strings.TrimSpace(req.Password) == "" {
		return BackupOutcome{}, fmt.Errorf("la contraseña del repositorio está vacía")
	}
	if len(req.Sources) == 0 {
		return BackupOutcome{}, fmt.Errorf("no hay carpetas de origen")
	}
	if err := locationAvailable(req.Repository); err != nil {
		return BackupOutcome{}, err
	}
	if !repositoryInitialized(req.Repository) {
		return BackupOutcome{}, fmt.Errorf("el repositorio no está inicializado. Ejecuta agent init-repo")
	}
	version, err := Version(ctx, req.Executable)
	if err != nil {
		return BackupOutcome{}, err
	}

	var parsed []Line
	code, _, stderr, runErr := run(ctx, req.Executable, req.Password, req.Repository, backupArgs(req), func(line Line) {
		parsed = append(parsed, line)
		if line.Status != nil && req.OnStatus != nil {
			req.OnStatus(*line.Status)
		}
	})
	if runErr != nil && ctx.Err() != nil {
		return BackupOutcome{
			Status:        StatusFailed,
			ExitCode:      code,
			Message:       "La copia se ha cancelado",
			ResticVersion: version,
		}, nil
	}
	if runErr != nil && code == 0 {
		return BackupOutcome{}, runErr
	}
	outcome := outcomeFrom(code, parsed, stderr, req.UseVSS)
	outcome.ResticVersion = version
	return outcome, nil
}

// InitRequest inicializa un repositorio nuevo.
type InitRequest struct {
	Executable string
	Repository string
	Password   string
}

// InitResult es el resultado de init-repo.
type InitResult struct {
	AlreadyExists bool
	ID            string
	Message       string
}

// Init crea el repositorio si todavía no existe.
func Init(ctx context.Context, req InitRequest) (InitResult, error) {
	if strings.TrimSpace(req.Password) == "" {
		return InitResult{}, fmt.Errorf("la contraseña del repositorio está vacía")
	}
	if err := locationAvailable(req.Repository); err != nil {
		return InitResult{}, err
	}
	if repositoryInitialized(req.Repository) {
		return InitResult{AlreadyExists: true, Message: "El repositorio ya existe"}, nil
	}
	var parsed []Line
	code, _, stderr, runErr := run(ctx, req.Executable, req.Password, req.Repository, []string{"init", "--json"}, func(line Line) {
		parsed = append(parsed, line)
	})
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
	return InitResult{}, fmt.Errorf("%s", userMessage(code, notice(parsed, stderr), false))
}

func backupArgs(req BackupRequest) []string {
	args := []string{"backup", "--json"}
	if req.Host != "" {
		args = append(args, "--host", req.Host)
	}
	if req.UseVSS {
		args = append(args, "--use-fs-snapshot")
	}
	for _, pattern := range req.Excludes {
		args = append(args, "--exclude", pattern)
	}
	return append(args, req.Sources...)
}

// command arranca restic.exe desde su propia carpeta.
// No se oculta la ventana: un proceso de consola escondido es una señal típica para un antivirus.
func command(ctx context.Context, exe string, args ...string) *exec.Cmd {
	if abs, err := filepath.Abs(exe); err == nil {
		exe = abs
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = filepath.Dir(exe)
	return cmd
}

func run(ctx context.Context, exe, password, repo string, args []string, onLine func(Line)) (int, string, string, error) {
	return runWithEnv(ctx, exe, password, repo, nil, args, onLine)
}

// runWithEnv es run con variables de entorno añadidas (por ejemplo la contraseña del repositorio
// de origen en `restic copy`). Los secretos van siempre por el entorno, nunca en los argumentos.
func runWithEnv(ctx context.Context, exe, password, repo string, extraEnv []string, args []string, onLine func(Line)) (int, string, string, error) {
	cmd := command(ctx, exe, args...)
	cmd.Env = append(childEnv(os.Environ(), password, repo), extraEnv...)
	cmd.WaitDelay = 3 * time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, "", "", fmt.Errorf("no se pudo leer la salida de restic: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 0, "", "", fmt.Errorf("no se pudo leer los errores de restic: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return 0, "", "", fmt.Errorf("no se pudo ejecutar restic: %w", err)
	}

	var (
		mu         sync.Mutex
		stdoutText strings.Builder
		stderrText strings.Builder
		wg         sync.WaitGroup
	)
	consume := func(r io.Reader, dest *strings.Builder) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 32*1024*1024)
		for sc.Scan() {
			text := sc.Text()
			line, ok := parseLine(text)
			mu.Lock()
			if strings.TrimSpace(text) != "" {
				if dest.Len() > 0 {
					dest.WriteByte('\n')
				}
				dest.WriteString(text)
			}
			if ok && onLine != nil {
				onLine(line)
			}
			mu.Unlock()
		}
	}
	wg.Add(2)
	go consume(stdout, &stdoutText)
	go consume(stderr, &stderrText)
	wg.Wait()

	waitErr := cmd.Wait()
	code := 0
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			return 0, stdoutText.String(), stderrText.String(), fmt.Errorf("restic no terminó correctamente: %w", waitErr)
		}
	}
	if ctx.Err() != nil {
		return code, stdoutText.String(), stderrText.String(), ctx.Err()
	}
	return code, stdoutText.String(), stderrText.String(), nil
}

func childEnv(base []string, password, repo string) []string {
	env := make([]string, 0, len(base)+2)
	for _, entry := range base {
		name, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(name) {
		case "RESTIC_PASSWORD", "RESTIC_PASSWORD_FILE", "RESTIC_PASSWORD_COMMAND", "RESTIC_NEW_PASSWORD", "RESTIC_NEW_PASSWORD_FILE", "RESTIC_REPOSITORY", "RESTIC_REPOSITORY_FILE",
			"RESTIC_FROM_PASSWORD", "RESTIC_FROM_PASSWORD_FILE", "RESTIC_FROM_PASSWORD_COMMAND", "RESTIC_FROM_REPOSITORY", "RESTIC_FROM_REPOSITORY_FILE", "RESTIC_FROM_KEY_HINT":
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "RESTIC_PASSWORD="+password, "RESTIC_REPOSITORY="+repo)
	return env
}

func repositoryInitialized(path string) bool {
	info, err := os.Stat(filepath.Join(path, "config"))
	return err == nil && !info.IsDir()
}

func locationAvailable(path string) error {
	current := filepath.Clean(path)
	for {
		info, err := os.Stat(current)
		if err == nil {
			if info.IsDir() {
				return nil
			}
			return fmt.Errorf("el destino no es una carpeta: %s", path)
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("Destino no disponible: conecta el disco USB o comprueba la ruta %s", path)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return fmt.Errorf("Destino no disponible: conecta el disco USB o comprueba la ruta %s", path)
		}
		current = parent
	}
}
