// Package state guarda la última copia y, más adelante, la última verificación.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const schemaVersion = 1

// State es el contenido de state.json.
type State struct {
	SchemaVersion int         `json:"schema_version"`
	LastBackup    *LastBackup `json:"last_backup,omitempty"`
	LastVerify    *LastVerify `json:"last_verify,omitempty"`
}

// LastBackup resume la ejecución más reciente.
type LastBackup struct {
	FinishedAt string `json:"finished_at"`
	Status     string `json:"status"`
	SnapshotID string `json:"snapshot_id,omitempty"`
	Report     string `json:"report"`
}

// LastVerify resume la última comprobación del repositorio.
type LastVerify struct {
	FinishedAt string `json:"finished_at"`
	Status     string `json:"status"`
	Report     string `json:"report"`
}

// Load lee el estado. Si el fichero no existe, devuelve un estado vacío.
func Load(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{SchemaVersion: schemaVersion}, nil
		}
		return State{}, fmt.Errorf("no se pudo leer el estado: %w", err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, fmt.Errorf("el estado no es un JSON válido: %w", err)
	}
	if st.SchemaVersion == 0 {
		st.SchemaVersion = schemaVersion
	}
	return st, nil
}

// Save escribe el estado de forma atómica.
func Save(path string, st State) error {
	if st.SchemaVersion == 0 {
		st.SchemaVersion = schemaVersion
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("no se pudo preparar el estado: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("no se pudo crear la carpeta de estado: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("no se pudo escribir el estado: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("no se pudo guardar el estado: %w", err2)
		}
	}
	return nil
}
