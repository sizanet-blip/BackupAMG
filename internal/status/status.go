// Package status resume el semáforo que muestra el panel.
package status

import "time"

const (
	// Green es una copia correcta y reciente.
	Green = "green"
	// Yellow es una copia con avisos o retrasada.
	Yellow = "yellow"
	// Red es un fallo o demasiado tiempo sin una copia correcta.
	Red = "red"
	// Blue es una copia o una verificación en curso.
	Blue = "blue"

	ActivityBackup  = "backup"
	ActivityVerify  = "verify"
	ActivityRestore = "restore"
)

// View es el estado que consumen la página de inicio y los eventos.
type View struct {
	Level       string         `json:"level"`
	Activity    string         `json:"activity"`
	AgentName   string         `json:"agent_name,omitempty"`
	LastBackup  *Run           `json:"last_backup"`
	NextBackup  string         `json:"next_backup,omitempty"`
	Destination Destination    `json:"destination"`
	LastVerify  *Run           `json:"last_verify"`
	Progress    *Progress      `json:"progress"`
	Restore     *RestoreResult `json:"restore,omitempty"`
	Job         *Job           `json:"job,omitempty"`
	Problem     string         `json:"problem,omitempty"`
}

// Job describe la tarea que está en curso.
type Job struct {
	Trigger   string `json:"trigger,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
	UseVSS    bool   `json:"use_vss,omitempty"`
}

// Run es la última copia o la última verificación.
type Run struct {
	FinishedAt      string `json:"finished_at"`
	Status          string `json:"status"`
	DurationSeconds int    `json:"duration_seconds"`
	DataAddedBytes  int64  `json:"data_added_bytes"`
	Message         string `json:"message,omitempty"`
	Detail          string `json:"detail,omitempty"`
}

// Destination es el repositorio y el espacio del disco.
type Destination struct {
	Path       string `json:"path"`
	Connected  bool   `json:"connected"`
	FreeBytes  int64  `json:"free_bytes"`
	TotalBytes int64  `json:"total_bytes"`
	UsedBytes  int64  `json:"used_bytes"`
	UsedKnown  bool   `json:"used_known"`
	Kind       string `json:"kind,omitempty"`
	Label      string `json:"label,omitempty"`
}

// RestoreResult es el último intento de recuperar ficheros.
type RestoreResult struct {
	Status  string   `json:"status"`
	Files   int      `json:"files"`
	Renamed int      `json:"renamed"`
	Target  string   `json:"target"`
	Message string   `json:"message"`
	Errors  []string `json:"errors"`
}

// Progress es el avance de restic durante una copia o una recuperación.
type Progress struct {
	Percent          float64 `json:"percent"`
	FilesDone        int     `json:"files_done"`
	FilesTotal       int     `json:"files_total"`
	BytesDone        int64   `json:"bytes_done"`
	BytesPerSecond   float64 `json:"bytes_per_second"`
	SecondsRemaining float64 `json:"seconds_remaining"`
	SecondsElapsed   float64 `json:"seconds_elapsed"`
	CurrentFile      string  `json:"current_file,omitempty"`
}

// Facts son los datos con los que se elige el color.
type Facts struct {
	Now        time.Time
	Activity   string
	Last       *Run
	StaleAfter time.Duration
	Overdue    bool
}

// Decide elige el color del semáforo.
// Una tarea en curso manda sobre el resto. Rojo es un fallo o más de StaleAfter
// sin una copia correcta. Amarillo es un aviso o una copia que ya tocaba.
func Decide(f Facts) string {
	if f.Activity == ActivityBackup || f.Activity == ActivityVerify || f.Activity == ActivityRestore {
		return Blue
	}
	if f.Last == nil || f.Last.Status == "" || f.Last.Status == "failed" {
		return Red
	}
	finished, err := time.Parse(time.RFC3339, f.Last.FinishedAt)
	if err != nil {
		return Red
	}
	tooOld := f.StaleAfter > 0 && f.Now.Sub(finished) > f.StaleAfter
	if f.Last.Status == "success" && tooOld {
		return Red
	}
	if f.Last.Status == "warning" && tooOld {
		return Red
	}
	if f.Last.Status == "warning" || f.Overdue {
		return Yellow
	}
	if f.Last.Status == "success" {
		return Green
	}
	return Red
}
