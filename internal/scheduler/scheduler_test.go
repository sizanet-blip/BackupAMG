package scheduler

import (
	"testing"
	"time"
)

func TestMostRecentDueWhenTheComputerWasOff(t *testing.T) {
	loc := time.FixedZone("CET", 3600)
	now := time.Date(2026, 11, 13, 21, 0, 0, 0, loc)
	last := time.Date(2026, 11, 12, 20, 10, 0, 0, loc)
	slot, ok := MostRecentDue(now, last, []string{"13:30", "20:00"})
	if !ok {
		t.Fatal("la copia de las 20:00 no se hizo")
	}
	if slot.Hour() != 20 || slot.Minute() != 0 || slot.Day() != 13 {
		t.Fatalf("hora atrasada: %s", slot)
	}
	if SameMinute(slot, now) {
		t.Fatal("una hora pasada no es la hora en punto")
	}
}

func TestNoDueWhenTheSlotWasAlreadyDone(t *testing.T) {
	loc := time.FixedZone("CET", 3600)
	now := time.Date(2026, 11, 13, 21, 0, 0, 0, loc)
	last := time.Date(2026, 11, 13, 20, 5, 0, 0, loc)
	if _, ok := MostRecentDue(now, last, []string{"13:30", "20:00"}); ok {
		t.Fatal("la copia de las 20:00 ya se hizo")
	}
}

func TestDueAtTheExactMinuteIsThatSlot(t *testing.T) {
	loc := time.FixedZone("CET", 3600)
	now := time.Date(2026, 11, 13, 13, 30, 20, 0, loc)
	last := time.Date(2026, 11, 12, 20, 0, 0, 0, loc)
	slot, ok := MostRecentDue(now, last, []string{"13:30", "20:00"})
	if !ok || !SameMinute(slot, now) {
		t.Fatalf("en el minuto de la hora debe tocar esa copia: %s %v", slot, ok)
	}
}

func TestFirstStartCatchesTheLastSlotInside24Hours(t *testing.T) {
	loc := time.FixedZone("CET", 3600)
	now := time.Date(2026, 11, 13, 9, 0, 0, 0, loc)
	slot, ok := MostRecentDue(now, time.Time{}, []string{"13:30", "20:00"})
	if !ok || slot.Day() != 12 || slot.Hour() != 20 {
		t.Fatalf("al arrancar por la mañana toca la copia de anoche: %s %v", slot, ok)
	}
}

func TestNextAfter(t *testing.T) {
	loc := time.FixedZone("CET", 3600)
	now := time.Date(2026, 11, 13, 15, 0, 0, 0, loc)
	next, err := NextAfter(now, []string{"20:00", "13:30"})
	if err != nil {
		t.Fatal(err)
	}
	if next.Hour() != 20 || next.Day() != 13 {
		t.Fatalf("siguiente: %s", next)
	}
	later := time.Date(2026, 11, 13, 21, 0, 0, 0, loc)
	next, err = NextAfter(later, []string{"13:30", "20:00"})
	if err != nil {
		t.Fatal(err)
	}
	if next.Day() != 14 || next.Hour() != 13 || next.Minute() != 30 {
		t.Fatalf("mañana: %s", next)
	}
	exact := time.Date(2026, 11, 13, 20, 0, 0, 0, loc)
	next, err = NextAfter(exact, []string{"13:30", "20:00"})
	if err != nil || next.Day() != 14 {
		t.Fatalf("en el minuto exacto la siguiente es mañana: %s %v", next, err)
	}
}

func TestNextAfterSkipsDaysThatAreNotSelected(t *testing.T) {
	loc := time.FixedZone("CET", 3600)
	friday := time.Date(2026, 11, 13, 21, 0, 0, 0, loc)
	next, err := NextAfterOn(friday, []string{"13:30"}, []int{1})
	if err != nil {
		t.Fatal(err)
	}
	if next.Weekday() != time.Monday || next.Day() != 16 || next.Hour() != 13 {
		t.Fatalf("el lunes siguiente: %s", next)
	}
}

func TestShouldRetry(t *testing.T) {
	if !ShouldRetry(1, 2) || !ShouldRetry(2, 2) || ShouldRetry(3, 2) {
		t.Fatal("con 2 reintentos hay tres intentos en total")
	}
	if ShouldRetry(1, 0) {
		t.Fatal("sin reintentos no se repite")
	}
}
