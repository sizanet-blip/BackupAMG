package verify

import (
	"testing"
	"time"
)

func TestDue(t *testing.T) {
	now := time.Date(2026, 11, 13, 20, 0, 0, 0, time.Local)
	if !Due(time.Time{}, 7, now) {
		t.Fatal("sin verificación previa debe tocar")
	}
	if Due(now.AddDate(0, 0, -6), 7, now) {
		t.Fatal("a los 6 días todavía no toca")
	}
	if !Due(now.AddDate(0, 0, -7), 7, now) {
		t.Fatal("a los 7 días toca")
	}
	if Due(now.AddDate(0, 0, -30), 0, now) {
		t.Fatal("un plazo menor que 1 día no dispara la verificación")
	}
}
