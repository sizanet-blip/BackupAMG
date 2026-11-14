package restic

import (
	"encoding/json"
	"strings"
)

// Status es un avance de la copia. PercentDone va de 0 a 1.
type Status struct {
	PercentDone      float64
	TotalFiles       int
	FilesDone        int
	TotalBytes       int64
	BytesDone        int64
	SecondsElapsed   float64
	SecondsRemaining float64
	CurrentFiles     []string
}

// Summary es la última línea de una copia que llegó a crear snapshot.
type Summary struct {
	FilesNew            int
	FilesChanged        int
	FilesUnmodified     int
	DataAdded           int64
	TotalFilesProcessed int
	TotalBytesProcessed int64
	TotalDuration       float64
	SnapshotID          string
}

// Line es un mensaje JSON de restic.
type Line struct {
	MessageType string
	Status      *Status
	Error       *SkippedFile
	Summary     *Summary
	ExitError   *struct {
		Code    int
		Message string
	}
	Initialized *struct {
		ID string
	}
}

func parseLine(text string) (Line, bool) {
	text = strings.TrimSpace(text)
	if text == "" || text[0] != '{' {
		return Line{}, false
	}
	var raw struct {
		MessageType         string          `json:"message_type"`
		Error               json.RawMessage `json:"error"`
		During              string          `json:"during"`
		Item                string          `json:"item"`
		Code                int             `json:"code"`
		Message             string          `json:"message"`
		PercentDone         float64         `json:"percent_done"`
		TotalFiles          int             `json:"total_files"`
		FilesDone           int             `json:"files_done"`
		TotalBytes          int64           `json:"total_bytes"`
		BytesDone           int64           `json:"bytes_done"`
		SecondsElapsed      float64         `json:"seconds_elapsed"`
		SecondsRemaining    float64         `json:"seconds_remaining"`
		CurrentFiles        []string        `json:"current_files"`
		FilesNew            int             `json:"files_new"`
		FilesChanged        int             `json:"files_changed"`
		FilesUnmodified     int             `json:"files_unmodified"`
		DataAdded           int64           `json:"data_added"`
		TotalFilesProcessed int             `json:"total_files_processed"`
		TotalBytesProc      int64           `json:"total_bytes_processed"`
		TotalDuration       float64         `json:"total_duration"`
		SnapshotID          string          `json:"snapshot_id"`
		ID                  string          `json:"id"`
	}
	if err := json.Unmarshal([]byte(text), &raw); err != nil || raw.MessageType == "" {
		return Line{}, false
	}
	line := Line{MessageType: raw.MessageType}
	switch raw.MessageType {
	case "status":
		line.Status = &Status{
			PercentDone:      raw.PercentDone,
			TotalFiles:       raw.TotalFiles,
			FilesDone:        raw.FilesDone,
			TotalBytes:       raw.TotalBytes,
			BytesDone:        raw.BytesDone,
			SecondsElapsed:   raw.SecondsElapsed,
			SecondsRemaining: raw.SecondsRemaining,
			CurrentFiles:     raw.CurrentFiles,
		}
	case "error":
		line.Error = &SkippedFile{Path: raw.Item, Reason: errorText(raw.Error)}
	case "summary":
		line.Summary = &Summary{
			FilesNew:            raw.FilesNew,
			FilesChanged:        raw.FilesChanged,
			FilesUnmodified:     raw.FilesUnmodified,
			DataAdded:           raw.DataAdded,
			TotalFilesProcessed: raw.TotalFilesProcessed,
			TotalBytesProcessed: raw.TotalBytesProc,
			TotalDuration:       raw.TotalDuration,
			SnapshotID:          raw.SnapshotID,
		}
	case "exit_error":
		line.ExitError = &struct {
			Code    int
			Message string
		}{Code: raw.Code, Message: raw.Message}
	case "initialized":
		line.Initialized = &struct{ ID string }{ID: raw.ID}
	}
	return line, true
}

func errorText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var obj struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return obj.Message
	}
	return ""
}

func outcomeFrom(exitCode int, lines []Line, stderr string, useVSS bool) BackupOutcome {
	out := BackupOutcome{ExitCode: exitCode}
	for _, line := range lines {
		if line.Error != nil && line.Error.Path != "" {
			out.Skipped = append(out.Skipped, *line.Error)
		}
		if line.Summary != nil {
			out.SnapshotID = line.Summary.SnapshotID
			out.FilesNew = line.Summary.FilesNew
			out.FilesChanged = line.Summary.FilesChanged
			out.FilesUnmodified = line.Summary.FilesUnmodified
			out.DataAdded = line.Summary.DataAdded
			out.TotalFilesProcessed = line.Summary.TotalFilesProcessed
			out.TotalBytesProcessed = line.Summary.TotalBytesProcessed
			out.DurationSeconds = line.Summary.TotalDuration
		}
	}
	switch exitCode {
	case 0:
		if out.SnapshotID == "" {
			out.Status = StatusFailed
			out.Message = "La copia ha fallado: restic terminó sin devolver el resumen de la copia"
			return out
		}
		out.Status = StatusSuccess
	case 3:
		out.Status = StatusWarning
	default:
		out.Status = StatusFailed
	}
	out.Message = userMessage(exitCode, notice(lines, stderr), useVSS && out.Status == StatusFailed)
	return out
}

func notice(lines []Line, stderr string) string {
	for _, line := range lines {
		if line.ExitError != nil && strings.TrimSpace(line.ExitError.Message) != "" {
			return cleanResticMessage(line.ExitError.Message)
		}
	}
	return cleanResticMessage(stderr)
}

func cleanResticMessage(message string) string {
	message = strings.TrimSpace(message)
	for _, prefix := range []string{"Fatal: ", "Warning: "} {
		message = strings.TrimPrefix(message, prefix)
	}
	return strings.TrimSpace(message)
}

func userMessage(exitCode int, resticMessage string, vssFailed bool) string {
	var msg string
	switch exitCode {
	case 0:
		msg = "Copia completada correctamente"
	case 3:
		msg = "Copia completada con avisos: algunos ficheros no se pudieron leer"
	case 10:
		msg = "Destino no disponible: conecta el disco USB o comprueba la ruta"
	case 11:
		msg = "El repositorio está bloqueado por otra copia"
	case 12:
		msg = "La contraseña del repositorio no es correcta"
	default:
		msg = "La copia ha fallado"
		if resticMessage != "" {
			msg += ": " + resticMessage
		} else if exitCode > 0 {
			msg += " (código " + itoa(exitCode) + ")"
		}
	}
	if vssFailed && vssHint(resticMessage) {
		msg += ". Para copiar ficheros abiertos hay que ejecutar el agente como administrador"
	}
	return msg
}

func vssHint(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "vss") || strings.Contains(lower, "shadow") || strings.Contains(lower, "instantánea")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [12]byte
	i := len(digits)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		digits[i] = '-'
	}
	return string(digits[i:])
}
