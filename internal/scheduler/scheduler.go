// Package scheduler decide cuándo toca hacer una copia.
package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MostRecentDue devuelve la hora programada más reciente que ya ha pasado
// y para la que no hay un intento posterior. Si lastAttempt es cero, solo
// mira las últimas 24 horas: una instalación nueva no repite días antiguos.
func MostRecentDue(now, lastAttempt time.Time, clocks []string) (time.Time, bool) {
	return mostRecentDue(now, lastAttempt, clocks, nil)
}

// MostRecentDueOn es MostRecentDue limitado a unos días. weekdays usa 1 (lunes) a 7 (domingo). Vacío significa todos.
func MostRecentDueOn(now, lastAttempt time.Time, clocks []string, weekdays []int) (time.Time, bool) {
	return mostRecentDue(now, lastAttempt, clocks, weekdays)
}

func mostRecentDue(now, lastAttempt time.Time, clocks []string, weekdays []int) (time.Time, bool) {
	now = now.In(now.Location())
	var best time.Time
	found := false
	startDay := now.Add(-48 * time.Hour)
	if !lastAttempt.IsZero() && lastAttempt.Before(startDay) {
		startDay = lastAttempt
	}
	day := time.Date(startDay.Year(), startDay.Month(), startDay.Day(), 0, 0, 0, 0, now.Location())
	lastDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for !day.After(lastDay) {
		for _, clock := range clocks {
			slot, err := onDay(day, clock)
			if err != nil || slot.After(now) || !dayAllowed(slot, weekdays) {
				continue
			}
			if lastAttempt.IsZero() && slot.Before(now.Add(-24*time.Hour)) {
				continue
			}
			if !lastAttempt.IsZero() && !slot.After(lastAttempt) {
				continue
			}
			if !found || slot.After(best) {
				best = slot
				found = true
			}
		}
		day = day.AddDate(0, 0, 1)
	}
	return best, found
}

// NextAfter es la próxima hora estrictamente posterior a now.
func NextAfter(now time.Time, clocks []string) (time.Time, error) {
	return nextAfter(now, clocks, nil)
}

// NextAfterOn es NextAfter limitado a unos días. weekdays usa 1 (lunes) a 7 (domingo). Vacío significa todos.
func NextAfterOn(now time.Time, clocks []string, weekdays []int) (time.Time, error) {
	return nextAfter(now, clocks, weekdays)
}

func nextAfter(now time.Time, clocks []string, weekdays []int) (time.Time, error) {
	if len(clocks) == 0 {
		return time.Time{}, fmt.Errorf("no hay horas de copia configuradas")
	}
	now = now.In(now.Location())
	var next time.Time
	for offset := 0; offset <= 7; offset++ {
		day := now.AddDate(0, 0, offset)
		day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, now.Location())
		for _, clock := range clocks {
			slot, err := onDay(day, clock)
			if err != nil {
				return time.Time{}, err
			}
			if !slot.After(now) || !dayAllowed(slot, weekdays) {
				continue
			}
			if next.IsZero() || slot.Before(next) {
				next = slot
			}
		}
	}
	if next.IsZero() {
		return time.Time{}, fmt.Errorf("no hay horas de copia configuradas")
	}
	return next, nil
}

// SameMinute indica si dos instantes caen en el mismo minuto local.
func SameMinute(a, b time.Time) bool {
	a = a.In(b.Location())
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day() && a.Hour() == b.Hour() && a.Minute() == b.Minute()
}

// ShouldRetry dice si, tras el intento attempt (empezando en 1), queda algún reintento.
func ShouldRetry(attempt, maxRetries int) bool {
	if maxRetries < 0 {
		return false
	}
	return attempt-1 < maxRetries
}

func dayAllowed(day time.Time, weekdays []int) bool {
	if len(weekdays) == 0 {
		return true
	}
	n := int(day.Weekday())
	if n == 0 {
		n = 7
	}
	for _, want := range weekdays {
		if want == n {
			return true
		}
	}
	return false
}

func onDay(day time.Time, clock string) (time.Time, error) {
	parts := strings.Split(clock, ":")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("hora no válida %q", clock)
	}
	hour, errH := strconv.Atoi(parts[0])
	minute, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return time.Time{}, fmt.Errorf("hora no válida %q", clock)
	}
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, day.Location()), nil
}
