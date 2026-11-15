package status

import (
	"testing"
	"time"
)

// Una copia adicional retrasada solo puede pasar el verde a amarillo: nunca a rojo.
func TestReplicaStaleOnlyTurnsGreenIntoYellow(t *testing.T) {
	now := time.Date(2026, 11, 13, 22, 0, 0, 0, time.Local)
	recent := now.Add(-time.Hour).Format(time.RFC3339)
	old := now.Add(-72 * time.Hour).Format(time.RFC3339)
	stale := 48 * time.Hour
	cases := []struct {
		name string
		in   Facts
		want string
	}{
		{"verde con adicional retrasada", Facts{Now: now, StaleAfter: stale, Last: run("success", recent), ReplicaStale: true}, Yellow},
		{"verde con adicional al día", Facts{Now: now, StaleAfter: stale, Last: run("success", recent)}, Green},
		{"la principal fallida sigue roja", Facts{Now: now, StaleAfter: stale, Last: run("failed", recent), ReplicaStale: true}, Red},
		{"la principal antigua sigue roja", Facts{Now: now, StaleAfter: stale, Last: run("success", old), ReplicaStale: true}, Red},
		{"en curso sigue azul", Facts{Now: now, Activity: ActivityBackup, Last: run("success", recent), ReplicaStale: true}, Blue},
		{"una réplica en curso no pone azul", Facts{Now: now, Activity: ActivityReplica, StaleAfter: stale, Last: run("success", recent)}, Green},
		{"una réplica en curso con la principal fallida es roja", Facts{Now: now, Activity: ActivityReplica, StaleAfter: stale, Last: run("failed", recent)}, Red},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Decide(tc.in); got != tc.want {
				t.Fatalf("color %s, se esperaba %s", got, tc.want)
			}
		})
	}
}

func TestDescribeReplica(t *testing.T) {
	now := time.Date(2026, 11, 13, 22, 0, 0, 0, time.Local)
	ago := func(h int) string { return now.Add(-time.Duration(h) * time.Hour).Format(time.RFC3339) }
	base := ReplicaFacts{ID: "usb", Name: "Disco", Enabled: true, Now: now, StaleAfter: 48 * time.Hour}
	with := func(f func(*ReplicaFacts)) ReplicaFacts { c := base; f(&c); return c }

	cases := []struct {
		name       string
		in         ReplicaFacts
		state      string
		stale      bool
		daysBehind int
	}{
		{"al día", with(func(f *ReplicaFacts) { f.LastSuccessAt = ago(1) }), ReplicaOK, false, 0},
		{"pausada", with(func(f *ReplicaFacts) { f.Enabled = false; f.Pending = true; f.LastSuccessAt = ago(500) }), ReplicaDisabled, false, 0},
		{"pendiente reciente", with(func(f *ReplicaFacts) { f.Pending = true; f.LastSuccessAt = ago(5) }), ReplicaPending, false, 0},
		{"pendiente retrasada", with(func(f *ReplicaFacts) { f.Pending = true; f.LastSuccessAt = ago(24 * 5) }), ReplicaPending, true, 5},
		{"sin conexión reciente", with(func(f *ReplicaFacts) { f.Pending = true; f.LastStatus = "offline"; f.LastSuccessAt = ago(5) }), ReplicaOffline, false, 0},
		{"sin conexión retrasada", with(func(f *ReplicaFacts) { f.Pending = true; f.LastStatus = "offline"; f.LastSuccessAt = ago(24 * 3) }), ReplicaOffline, true, 3},
		{"fallo con causa", with(func(f *ReplicaFacts) {
			f.Pending = true
			f.LastStatus = "failed"
			f.LastError = "No hay sitio"
			f.LastSuccessAt = ago(2)
		}), ReplicaFailed, false, 0},
		{"nunca replicada, recién añadida", with(func(f *ReplicaFacts) { f.Pending = true; f.AddedAt = ago(3) }), ReplicaPending, false, 0},
		{"nunca replicada, añadida hace días", with(func(f *ReplicaFacts) { f.Pending = true; f.AddedAt = ago(24 * 4) }), ReplicaPending, true, 4},
		{"interrumpida sigue como pendiente", with(func(f *ReplicaFacts) { f.Pending = true; f.LastStatus = "interrupted"; f.LastSuccessAt = ago(3) }), ReplicaPending, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DescribeReplica(tc.in)
			if got.State != tc.state || got.Stale != tc.stale || got.DaysBehind != tc.daysBehind {
				t.Fatalf("%+v", got)
			}
		})
	}
	if got := DescribeReplica(with(func(f *ReplicaFacts) { f.Pending = true; f.LastStatus = "failed"; f.LastError = "No hay sitio" })); got.Message != "No hay sitio" {
		t.Fatalf("falta la causa: %+v", got)
	}
}
