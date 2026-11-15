package cloud

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Estados de la conexión con la plataforma, tal como los ve el usuario.
const (
	StateNotConfigured  = "not_configured"  // no hay dirección del servidor
	StateNotEnrolled    = "not_enrolled"    // hay servidor pero el equipo no está dado de alta
	StateConnected      = "connected"       // todo bien
	StateOffline        = "offline"         // no se pudo hablar con la plataforma
	StateLicenseExpired = "license_expired" // licencia caducada
	StateLicenseBlocked = "license_blocked" // licencia suspendida, revocada o sin empezar
	StateSeatLimit      = "seat_limit"      // límite de equipos
	StateRevoked        = "revoked"         // el técnico desactivó este equipo
	StateNeedsEnroll    = "needs_enroll"    // la clave del equipo ya no vale: hay que dar de alta de nuevo
)

// maxQueue es el tope de informes pendientes que se recuerdan.
const maxQueue = 5000

// QueueItem es un informe pendiente de enviar. Solo se guarda el nombre del fichero en reports\.
type QueueItem struct {
	File string `json:"file"`
}

// Data es lo que se guarda en cloud.json. La clave del equipo va cifrada con DPAPI.
type Data struct {
	Version      int    `json:"version"`
	KeyEncrypted string `json:"machine_key_encrypted,omitempty"`
	// ServerURL es el servidor con el que se hizo el alta.
	ServerURL   string      `json:"server_url,omitempty"`
	CompanyName string      `json:"company_name,omitempty"`
	EnrolledAt  string      `json:"enrolled_at,omitempty"`
	License     LicenseInfo `json:"license"`

	State        string `json:"state,omitempty"`
	StateCode    string `json:"state_code,omitempty"` // último código de negocio
	StateMessage string `json:"state_message,omitempty"`
	StateAt      string `json:"state_at,omitempty"`

	LastHeartbeatAt string `json:"last_heartbeat_at,omitempty"`
	LastReportsAt   string `json:"last_reports_at,omitempty"`
	IntervalSeconds int    `json:"interval_seconds,omitempty"`
	NextAttempt     string `json:"next_attempt,omitempty"`
	Failures        int    `json:"failures,omitempty"`

	MinAgentVersion         string `json:"min_agent_version,omitempty"`
	RecommendedAgentVersion string `json:"recommended_agent_version,omitempty"`

	Queue    []QueueItem `json:"queue,omitempty"`
	Rejected []string    `json:"rejected,omitempty"` // últimos informes rechazados (nombre y código)
}

const storeVersion = 1

// Store guarda cloud.json de forma atómica.
type Store struct {
	path string
	mu   sync.Mutex
	data Data
}

// OpenStore lee cloud.json. Si no existe, empieza vacío.
func OpenStore(path string) (*Store, error) {
	s := &Store{path: path}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.data.Version = storeVersion
			return s, nil
		}
		return nil, fmt.Errorf("no se pudo leer el estado de la plataforma: %w", err)
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return nil, fmt.Errorf("el estado de la plataforma no es válido: %w", err)
	}
	if s.data.Version == 0 {
		s.data.Version = storeVersion
	}
	return s, nil
}

// Snapshot devuelve una copia de los datos.
func (s *Store) Snapshot() Data {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.copyLocked()
}

func (s *Store) copyLocked() Data {
	d := s.data
	d.Queue = append([]QueueItem(nil), s.data.Queue...)
	d.Rejected = append([]string(nil), s.data.Rejected...)
	return d
}

// Update cambia los datos bajo el candado y los guarda. Si guardar falla, devuelve el error pero
// los cambios siguen en memoria: el agente nunca se detiene por esto.
func (s *Store) Update(change func(*Data)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	change(&s.data)
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(s.path)
		if err2 := os.Rename(tmp, s.path); err2 != nil {
			_ = os.Remove(tmp)
			return err2
		}
	}
	return nil
}

// Enqueue añade un informe a la cola (una sola vez) y recorta si hay demasiados.
func (s *Store) Enqueue(file string) error {
	return s.Update(func(d *Data) {
		for _, item := range d.Queue {
			if item.File == file {
				return
			}
		}
		d.Queue = append(d.Queue, QueueItem{File: file})
		if over := len(d.Queue) - maxQueue; over > 0 {
			d.Queue = d.Queue[over:]
		}
	})
}

// Pending es el número de informes sin enviar.
func (s *Store) Pending() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.data.Queue)
}
