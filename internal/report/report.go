// Package report construye el JSON de cada ejecución.
// El formato es estable: lo consumirá la plataforma web.
package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// SchemaVersion es la versión del JSON. Subirla si el formato deja de ser compatible.
	SchemaVersion = 1

	TypeBackup = "backup"
	TypeVerify = "verify"
	// TypeReplica es la réplica del repositorio principal en una copia adicional.
	TypeReplica = "replica"

	TriggerScheduled       = "scheduled"
	TriggerMissedOnStartup = "missed_on_startup"
	TriggerManual          = "manual"
	TriggerRetry           = "retry"
)

// Report es el informe que se guarda en reports y, más adelante, se enviará a la web.
type Report struct {
	SchemaVersion        int           `json:"schema_version"`
	Type                 string        `json:"type"`
	AgentVersion         string        `json:"agent_version"`
	ResticVersion        string        `json:"restic_version"`
	Hostname             string        `json:"hostname"`
	StartedAt            string        `json:"started_at"`
	FinishedAt           string        `json:"finished_at"`
	DurationSeconds      int           `json:"duration_seconds"`
	Status               string        `json:"status"`
	Trigger              string        `json:"trigger"`
	Attempt              int           `json:"attempt"`
	Sources              []string      `json:"sources"`
	Repository           string        `json:"repository"`
	VSSUsed              bool          `json:"vss_used"`
	FilesNew             int           `json:"files_new"`
	FilesChanged         int           `json:"files_changed"`
	FilesUnmodified      int           `json:"files_unmodified"`
	DataAddedBytes       int64         `json:"data_added_bytes"`
	TotalFilesProcessed  int           `json:"total_files_processed"`
	TotalBytesProcessed  int64         `json:"total_bytes_processed"`
	SnapshotID           string        `json:"snapshot_id"`
	DestinationFreeBytes int64         `json:"destination_free_bytes"`
	SkippedFiles         []SkippedFile `json:"skipped_files"`
	Errors               []string      `json:"errors"`
	Retention            Retention     `json:"retention"`
	Message              string        `json:"message"`
	// Solo en los informes de réplica (type "replica"); en los demás no aparecen.
	ReplicaID       string `json:"replica_id,omitempty"`
	ReplicaName     string `json:"replica_name,omitempty"`
	SnapshotsCopied int    `json:"snapshots_copied,omitempty"`
}

// SkippedFile es un fichero que no se pudo leer.
type SkippedFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Retention indica si ya se aplicó la política de conservación.
// En esta fase la copia todavía no borra snapshots: Applied queda a false.
type Retention struct {
	Applied          bool `json:"applied"`
	SnapshotsRemoved int  `json:"snapshots_removed"`
}

// Input son los datos de una ejecución, antes de darles forma de informe.
type Input struct {
	AgentVersion         string
	ResticVersion        string
	Hostname             string
	Started              time.Time
	Finished             time.Time
	Status               string
	Type                 string
	Trigger              string
	Attempt              int
	Sources              []string
	Repository           string
	VSSUsed              bool
	FilesNew             int
	FilesChanged         int
	FilesUnmodified      int
	DataAddedBytes       int64
	TotalFilesProcessed  int
	TotalBytesProcessed  int64
	SnapshotID           string
	DestinationFreeBytes int64
	SkippedFiles         []SkippedFile
	Errors               []string
	Message              string
	RetentionApplied     bool
	SnapshotsRemoved     int
	ReplicaID            string
	ReplicaName          string
	SnapshotsCopied      int
}

// Build monta el informe. Las listas vacías se guardan como [] y no como null.
func Build(in Input) Report {
	started := in.Started.Local()
	finished := in.Finished.Local()
	if finished.Before(started) {
		finished = started
	}
	seconds := int(finished.Sub(started).Round(time.Second).Seconds())
	if seconds < 0 {
		seconds = 0
	}
	attempt := in.Attempt
	if attempt < 1 {
		attempt = 1
	}
	skipped := in.SkippedFiles
	if skipped == nil {
		skipped = []SkippedFile{}
	}
	errs := in.Errors
	if errs == nil {
		errs = []string{}
	}
	sources := in.Sources
	if sources == nil {
		sources = []string{}
	}
	kind := in.Type
	if kind == "" {
		kind = TypeBackup
	}
	return Report{
		SchemaVersion:        SchemaVersion,
		Type:                 kind,
		AgentVersion:         in.AgentVersion,
		ResticVersion:        in.ResticVersion,
		Hostname:             in.Hostname,
		StartedAt:            started.Format(time.RFC3339),
		FinishedAt:           finished.Format(time.RFC3339),
		DurationSeconds:      seconds,
		Status:               in.Status,
		Trigger:              in.Trigger,
		Attempt:              attempt,
		Sources:              sources,
		Repository:           in.Repository,
		VSSUsed:              in.VSSUsed,
		FilesNew:             in.FilesNew,
		FilesChanged:         in.FilesChanged,
		FilesUnmodified:      in.FilesUnmodified,
		DataAddedBytes:       in.DataAddedBytes,
		TotalFilesProcessed:  in.TotalFilesProcessed,
		TotalBytesProcessed:  in.TotalBytesProcessed,
		SnapshotID:           in.SnapshotID,
		DestinationFreeBytes: in.DestinationFreeBytes,
		SkippedFiles:         skipped,
		Errors:               errs,
		Retention: Retention{
			Applied:          in.RetentionApplied,
			SnapshotsRemoved: in.SnapshotsRemoved,
		},
		Message:         in.Message,
		ReplicaID:       in.ReplicaID,
		ReplicaName:     in.ReplicaName,
		SnapshotsCopied: in.SnapshotsCopied,
	}
}

// FileName es el nombre del JSON, ordenable y válido en Windows.
func FileName(started time.Time) string {
	return started.Local().Format("2006-01-02_15-04-05.000") + ".json"
}

// Write guarda el informe en dir y devuelve la ruta completa.
// started fija el nombre del fichero; se usa el instante real y no el texto ya redondeado.
func Write(dir string, rep Report, started time.Time) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("no se pudo crear la carpeta de informes: %w", err)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rep); err != nil {
		return "", fmt.Errorf("no se pudo preparar el informe: %w", err)
	}
	path := filepath.Join(dir, FileName(started))
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("no se pudo escribir el informe: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			_ = os.Remove(tmp)
			return "", fmt.Errorf("no se pudo guardar el informe: %w", err2)
		}
	}
	return path, nil
}
