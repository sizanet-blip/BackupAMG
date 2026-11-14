// Package housekeeping borra informes y logs que ya tienen más de KeepDays días.
package housekeeping

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// KeepDays es el tiempo que se conservan informes y copias antiguas del log.
const KeepDays = 90

// Prune elimina informes y logs rotados anteriores al plazo. No toca el log activo.
func Prune(reportsDir, logsDir string, now time.Time) (int, error) {
	cutoff := now.AddDate(0, 0, -KeepDays)
	removed, err := pruneDir(reportsDir, cutoff, false)
	if err != nil {
		return removed, err
	}
	logs, err := pruneDir(logsDir, cutoff, true)
	return removed + logs, err
}

func pruneDir(dir string, cutoff time.Time, keepActiveLog bool) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("no se pudo leer %s: %w", dir, err)
	}
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if keepActiveLog && name == "agent.log" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return removed, err
		}
		when := info.ModTime()
		if parsed, ok := reportStamp(name); ok {
			when = parsed
		}
		if !when.Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return removed, fmt.Errorf("no se pudo borrar %s: %w", name, err)
		}
		removed++
	}
	return removed, nil
}

func reportStamp(name string) (time.Time, bool) {
	base := strings.TrimSuffix(name, ".json")
	if base == name {
		return time.Time{}, false
	}
	parsed, err := time.ParseInLocation("2006-01-02_15-04-05.000", base, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}
