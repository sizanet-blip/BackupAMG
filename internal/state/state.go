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
	// LastGoodSnapshot es la instantánea de la última copia correcta o con avisos. Una copia
	// fallida posterior no la borra: es lo que las copias adicionales tienen que replicar.
	LastGoodSnapshot string `json:"last_good_snapshot,omitempty"`
	// Replicas es el estado de cada copia adicional, por identificador. Opcional.
	Replicas map[string]*ReplicaState `json:"replicas,omitempty"`
	// PendingKeyRemovals son contraseñas antiguas que tras un cambio de contraseña todavía abren
	// algún repositorio y hay que quitar. No llevan ninguna contraseña, solo identificadores.
	PendingKeyRemovals []KeyRemoval `json:"pending_key_removals,omitempty"`
}

// KeyRemoval es una clave antigua pendiente de quitar. Replica vacío es la copia principal.
type KeyRemoval struct {
	Replica string `json:"replica,omitempty"`
	KeyID   string `json:"key_id"`
}

// Resultados de una réplica.
const (
	ReplicaOK          = "success"
	ReplicaFailed      = "failed"
	ReplicaOffline     = "offline"
	ReplicaInterrupted = "interrupted"
)

// ReplicaState es lo que se sabe de una copia adicional.
type ReplicaState struct {
	// AddedAt es cuándo se empezó a seguir esta copia adicional; sirve para saber si va retrasada
	// cuando todavía no se ha replicado nunca.
	AddedAt string `json:"added_at,omitempty"`
	// LastSuccessAt es la última réplica correcta. Vacío si nunca se ha hecho.
	LastSuccessAt string `json:"last_success_at,omitempty"`
	// LastSnapshot es la instantánea principal más reciente que ya está en la copia adicional.
	LastSnapshot string `json:"last_snapshot,omitempty"`
	LastAttempt  string `json:"last_attempt_at,omitempty"`
	LastStatus   string `json:"last_status,omitempty"`
	LastError    string `json:"last_error,omitempty"`
	Report       string `json:"report,omitempty"`
	// Interrupted cuenta las veces seguidas que la copia principal la ha interrumpido.
	Interrupted int `json:"interrupted,omitempty"`
}

// GoodSnapshot es la instantánea principal que las copias adicionales deben tener.
func (s State) GoodSnapshot() string {
	if s.LastGoodSnapshot != "" {
		return s.LastGoodSnapshot
	}
	if s.LastBackup != nil && s.LastBackup.Status != "failed" {
		return s.LastBackup.SnapshotID
	}
	return ""
}

// ReplicaPending dice si a la copia adicional le falta la última copia principal.
func (s State) ReplicaPending(id string) bool {
	good := s.GoodSnapshot()
	if good == "" {
		return false
	}
	r := s.Replicas[id]
	return r == nil || r.LastSnapshot != good
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
