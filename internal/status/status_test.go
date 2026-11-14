package status

import (
	"testing"
	"time"
)

func TestDecide(t *testing.T) {
	now := time.Date(2026, 11, 13, 22, 0, 0, 0, time.Local)
	recent := now.Add(-time.Hour).Format(time.RFC3339)
	old := now.Add(-72 * time.Hour).Format(time.RFC3339)
	stale := 48 * time.Hour

	cases := []struct {
		name string
		in   Facts
		want string
	}{
		{name: "en curso", in: Facts{Now: now, Activity: ActivityBackup, Last: run("failed", recent)}, want: Blue},
		{name: "verificando", in: Facts{Now: now, Activity: ActivityVerify}, want: Blue},
		{name: "sin copias", in: Facts{Now: now, StaleAfter: stale}, want: Red},
		{name: "fallo reciente", in: Facts{Now: now, StaleAfter: stale, Last: run("failed", recent)}, want: Red},
		{name: "correcta antigua", in: Facts{Now: now, StaleAfter: stale, Last: run("success", old)}, want: Red},
		{name: "aviso antiguo", in: Facts{Now: now, StaleAfter: stale, Last: run("warning", old)}, want: Red},
		{name: "aviso reciente", in: Facts{Now: now, StaleAfter: stale, Last: run("warning", recent)}, want: Yellow},
		{name: "retrasada", in: Facts{Now: now, StaleAfter: stale, Overdue: true, Last: run("success", recent)}, want: Yellow},
		{name: "al día", in: Facts{Now: now, StaleAfter: stale, Last: run("success", recent)}, want: Green},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Decide(tc.in); got != tc.want {
				t.Fatalf("color %s, se esperaba %s", got, tc.want)
			}
		})
	}
}

func run(state, at string) *Run {
	return &Run{Status: state, FinishedAt: at}
}
