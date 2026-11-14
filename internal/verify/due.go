// Package verify decide cuándo toca comprobar el repositorio.
package verify

import "time"

// Due indica si ya pasó el plazo desde la última verificación.
// Una fecha cero significa que nunca se ha verificado.
func Due(last time.Time, everyDays int, now time.Time) bool {
	if everyDays < 1 {
		return false
	}
	if last.IsZero() {
		return true
	}
	dueAt := last.AddDate(0, 0, everyDays)
	return !now.Before(dueAt)
}
