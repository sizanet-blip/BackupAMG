package restic

import (
	"os"
	"testing"
)

// Las pruebas usan una carpeta de datos propia: los ficheros temporales con contraseñas se crean
// dentro de la carpeta de datos del agente, nunca en la carpeta TEMP del sistema.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ba-restic-test-")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("BACKUP_AGENT_DATA_DIR", dir)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
