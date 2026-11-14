package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Summary es una línea del historial, sin el detalle.
type Summary struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	StartedAt       string `json:"started_at"`
	FinishedAt      string `json:"finished_at"`
	Status          string `json:"status"`
	Trigger         string `json:"trigger,omitempty"`
	DurationSeconds int    `json:"duration_seconds"`
	DataAddedBytes  int64  `json:"data_added_bytes"`
	Skipped         int    `json:"skipped,omitempty"`
	Message         string `json:"message,omitempty"`
}

// Filter acota el historial. Las fechas vacías no acotan.
type Filter struct {
	Status string
	From   time.Time
	To     time.Time
}

var reportID = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}_[0-9]{2}-[0-9]{2}-[0-9]{2}\.[0-9]{3}$`)

// ParseFilter interpreta el resultado y las fechas AAAA-MM-DD.
func ParseFilter(status, from, to string) (Filter, error) {
	status = strings.TrimSpace(status)
	switch status {
	case "", "success", "warning", "failed":
	default:
		return Filter{}, fmt.Errorf("el resultado no es válido")
	}
	var filter Filter
	filter.Status = status
	var err error
	if filter.From, err = parseDay(from, false); err != nil {
		return Filter{}, fmt.Errorf("la fecha inicial no es válida")
	}
	if filter.To, err = parseDay(to, true); err != nil {
		return Filter{}, fmt.Errorf("la fecha final no es válida")
	}
	if !filter.From.IsZero() && !filter.To.IsZero() && filter.From.After(filter.To) {
		return Filter{}, fmt.Errorf("la fecha inicial es posterior a la final")
	}
	return filter, nil
}

func parseDay(value string, end bool) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	day, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return time.Time{}, err
	}
	if end {
		return day.Add(24*time.Hour - time.Nanosecond), nil
	}
	return day, nil
}

// List lee los informes de dir, del más reciente al más antiguo.
func List(dir string, filter Filter) ([]Summary, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Summary{}, nil
		}
		return nil, fmt.Errorf("no se pudo leer el historial: %w", err)
	}
	var items []Summary
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !reportID.MatchString(id) {
			continue
		}
		rep, err := Open(dir, id)
		if err != nil {
			continue
		}
		finished, err := time.Parse(time.RFC3339, rep.FinishedAt)
		if err != nil {
			continue
		}
		finished = finished.In(time.Local)
		if filter.Status != "" && rep.Status != filter.Status {
			continue
		}
		if !filter.From.IsZero() && finished.Before(filter.From) {
			continue
		}
		if !filter.To.IsZero() && finished.After(filter.To) {
			continue
		}
		items = append(items, Summary{
			ID:              id,
			Type:            rep.Type,
			StartedAt:       rep.StartedAt,
			FinishedAt:      rep.FinishedAt,
			Status:          rep.Status,
			Trigger:         rep.Trigger,
			DurationSeconds: rep.DurationSeconds,
			DataAddedBytes:  rep.DataAddedBytes,
			Skipped:         len(rep.SkippedFiles),
			Message:         rep.Message,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].FinishedAt > items[j].FinishedAt
	})
	if items == nil {
		items = []Summary{}
	}
	return items, nil
}

// Open lee un informe por su identificador. No acepta rutas.
func Open(dir, id string) (Report, error) {
	if !reportID.MatchString(id) {
		return Report{}, fmt.Errorf("no se encontró esa ejecución")
	}
	data, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		return Report{}, fmt.Errorf("no se encontró esa ejecución")
	}
	var rep Report
	if err := json.Unmarshal(data, &rep); err != nil {
		return Report{}, fmt.Errorf("el informe no se puede leer")
	}
	return rep, nil
}
