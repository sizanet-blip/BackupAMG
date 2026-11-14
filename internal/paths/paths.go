// Package paths resuelve las rutas de datos del agente en el equipo.
package paths

import (
	"fmt"
	"os"
	"path/filepath"

	"backup-agent/internal/app"
	"backup-agent/internal/system"
)

// ProgramDataRoot es la carpeta real del equipo, sin tener en cuenta BACKUP_AGENT_DATA_DIR.
func ProgramDataRoot() string {
	programData := os.Getenv("ProgramData")
	if programData == "" {
		programData = `C:\ProgramData`
	}
	return filepath.Join(programData, app.Name)
}

// Overridden indica si las pruebas están usando otra carpeta.
func Overridden() bool {
	return os.Getenv(app.DataDirEnv) != ""
}

// DataDir es la raíz de configuración, informes, logs y estado.
func DataDir() string {
	if dir := os.Getenv(app.DataDirEnv); dir != "" {
		return dir
	}
	return ProgramDataRoot()
}

// ConfigPath es el fichero YAML de configuración.
func ConfigPath() string {
	return filepath.Join(DataDir(), "config.yaml")
}

// ReportsDir guarda un JSON por ejecución.
func ReportsDir() string {
	return filepath.Join(DataDir(), "reports")
}

// LogsDir guarda el log de texto del agente.
func LogsDir() string {
	return filepath.Join(DataDir(), "logs")
}

// StatePath guarda la última copia, la última verificación y datos afines.
func StatePath() string {
	return filepath.Join(DataDir(), "state.json")
}

// LogFile es el fichero de log activo.
func LogFile() string {
	return filepath.Join(LogsDir(), "agent.log")
}

// Ensure crea las carpetas de datos si no existen.
func Ensure() error {
	dirs := []string{DataDir(), ReportsDir(), LogsDir()}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("no se pudo crear el directorio %s: %w", dir, err)
		}
	}
	if !Overridden() {
		if err := system.ProtectTree(DataDir()); err != nil {
			return err
		}
	}
	return nil
}
