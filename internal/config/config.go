// Package config carga y valida el YAML del agente.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config es la configuración ya validada, con los valores por defecto aplicados.
type Config struct {
	Agent      Agent
	Backup     Backup
	Repository Repository
	Schedule   Schedule
	Retention  Retention
	Verify     Verify
	Panel      Panel
	// Replicas son las copias adicionales (opcional). Vacío: todo funciona como siempre.
	Replicas []Replica
}

// Agent identifica el equipo en los informes.
type Agent struct {
	Name string
}

// Backup describe qué carpetas entran en la copia.
type Backup struct {
	Sources  []string
	Excludes []string
	UseVSS   bool
}

// Repository es el destino de restic.
type Repository struct {
	Path              string
	PasswordEncrypted string
	// VolumeID identifica un disco local o USB. Vacío en una carpeta de red.
	VolumeID string
}

// Schedule decide cuándo se lanza la copia.
type Schedule struct {
	Times              []string
	RunMissedOnStartup bool
	MaxRetries         int
	RetryDelayMinutes  int
	// StaleAfterDays es el número de días sin una copia correcta para marcar el estado en rojo.
	StaleAfterDays int
	// Weekdays vale 1 (lunes) a 7 (domingo). Vacío significa todos los días.
	Weekdays []int
}

// Retention es la política de conservación de snapshots.
type Retention struct {
	KeepDaily   int
	KeepWeekly  int
	KeepMonthly int
	KeepYearly  int
	Prune       bool
}

// Verify es la comprobación periódica del repositorio.
type Verify struct {
	EveryDays      int
	ReadDataSubset string
}

// Panel es el puerto del interfaz local. Siempre escucha en 127.0.0.1.
type Panel struct {
	Port int
}

// DefaultPanelPort es el puerto del panel si la configuración no dice otro.
const DefaultPanelPort = 47800

// ValidationError reúne todos los problemas de un fichero, para corregirlos de una vez.
type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Error() string {
	var b strings.Builder
	b.WriteString("La configuración no es válida:")
	for _, problem := range e.Problems {
		b.WriteString("\n- ")
		b.WriteString(problem)
	}
	return b.String()
}

// Load lee el YAML de path, lo valida y aplica los valores por defecto.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("no se encontró la configuración en %s. Copia configs\\config.example.yaml a esa ruta y ajústala", path)
		}
		return nil, fmt.Errorf("no se pudo leer la configuración %s: %w", path, err)
	}
	return Parse(data)
}

// Parse valida el contenido YAML. No lee disco.
func Parse(data []byte) (*Config, error) {
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)

	var raw rawFile
	if err := dec.Decode(&raw); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, &ValidationError{Problems: []string{"el fichero está vacío"}}
		}
		return nil, wrapYAMLError(err)
	}

	var extra yaml.Node
	if err := dec.Decode(&extra); err == nil {
		return nil, &ValidationError{Problems: []string{"el fichero tiene más de un documento YAML"}}
	} else if !errors.Is(err, io.EOF) {
		return nil, wrapYAMLError(err)
	}
	return raw.validate()
}

func wrapYAMLError(err error) error {
	msg := strings.TrimPrefix(err.Error(), "yaml: unmarshal errors:\n")
	msg = strings.TrimPrefix(msg, "yaml: ")
	if strings.Contains(err.Error(), "not found in type") {
		return fmt.Errorf("la configuración tiene campos desconocidos o mal situados:\n%s", strings.TrimSpace(msg))
	}
	return fmt.Errorf("el fichero de configuración no es un YAML válido: %s", strings.TrimSpace(msg))
}
