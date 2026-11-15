package main

import (
	"context"
	"testing"
	"time"

	"backup-agent/internal/config"
	"backup-agent/internal/state"
	"backup-agent/internal/status"
)

func replicaState(snapshot string, rs *state.ReplicaState) state.State {
	st := state.State{LastGoodSnapshot: snapshot}
	if rs != nil {
		st.Replicas = map[string]*state.ReplicaState{"usb": rs}
	}
	return st
}

func TestReplicaDue(t *testing.T) {
	now := time.Date(2026, 11, 15, 14, 0, 0, 0, time.Local)
	hours := func(h float64) time.Duration { return time.Duration(h * float64(time.Hour)) }
	after := config.Replica{ID: "usb", Enabled: true, When: config.ReplicaAfterBackup}
	daily := config.Replica{ID: "usb", Enabled: true, When: config.ReplicaDailyAt, Time: "13:00"}

	cases := []struct {
		name string
		r    config.Replica
		st   state.State
		next time.Duration
		want bool
	}{
		{"pendiente tras una copia", after, replicaState("s2", &state.ReplicaState{LastSnapshot: "s1", LastStatus: state.ReplicaOK}), hours(5), true},
		{"al día", after, replicaState("s1", &state.ReplicaState{LastSnapshot: "s1", LastStatus: state.ReplicaOK}), hours(5), false},
		{"desactivada", config.Replica{ID: "usb", When: config.ReplicaAfterBackup}, replicaState("s2", nil), hours(5), false},
		{"sin copia principal correcta", after, state.State{}, hours(5), false},
		{"nueva, nunca replicada", after, replicaState("s1", nil), hours(5), true},
		{"la principal toca en 5 minutos", after, replicaState("s2", nil), 5 * time.Minute, false},
		{"la principal toca en 20 minutos", after, replicaState("s2", nil), 20 * time.Minute, true},
		{"interrumpida 3 veces espera un hueco largo", after, replicaState("s2", &state.ReplicaState{Interrupted: 3, LastStatus: state.ReplicaInterrupted}), 30 * time.Minute, false},
		{"interrumpida 3 veces con hueco largo", after, replicaState("s2", &state.ReplicaState{Interrupted: 3, LastStatus: state.ReplicaInterrupted}), hours(3), true},
		{"fallo reciente espera", after, replicaState("s2", &state.ReplicaState{LastStatus: state.ReplicaFailed, LastAttempt: now.Add(-10 * time.Minute).Format(time.RFC3339)}), hours(5), false},
		{"fallo antiguo reintenta", after, replicaState("s2", &state.ReplicaState{LastStatus: state.ReplicaFailed, LastAttempt: now.Add(-time.Hour).Format(time.RFC3339)}), hours(5), true},
		{"sin conexión se mira siempre", after, replicaState("s2", &state.ReplicaState{LastStatus: state.ReplicaOffline, LastAttempt: now.Add(-time.Minute).Format(time.RFC3339)}), hours(5), true},
		{"diaria antes de su hora", config.Replica{ID: "usb", Enabled: true, When: config.ReplicaDailyAt, Time: "15:00"}, replicaState("s2", nil), hours(5), false},
		{"diaria a su hora", daily, replicaState("s2", nil), hours(5), true},
		{"diaria ya intentada hoy y fallida", daily, replicaState("s2", &state.ReplicaState{LastStatus: state.ReplicaFailed, LastAttempt: now.Add(-time.Hour).Format(time.RFC3339)}), hours(5), false},
		{"diaria sin conexión se reintenta", daily, replicaState("s2", &state.ReplicaState{LastStatus: state.ReplicaOffline, LastAttempt: now.Add(-time.Hour).Format(time.RFC3339)}), hours(5), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := replicaDue(tc.r, tc.st, now, tc.next); got != tc.want {
				t.Fatalf("replicaDue = %v, quería %v", got, tc.want)
			}
		})
	}
}

// La copia principal tiene prioridad: cancela una réplica en curso, pero no a otras tareas.
func TestPreemptOnlyCancelsReplicas(t *testing.T) {
	var l liveJob
	ctx, stop, ok := l.enter(context.Background(), status.ActivityReplica)
	if !ok {
		t.Fatal("no entró")
	}
	if l.cancelKind(status.ActivityVerify) {
		t.Fatal("no debe cancelar una tarea de otro tipo")
	}
	if ctx.Err() != nil {
		t.Fatal("la réplica se canceló sin motivo")
	}
	if !l.cancelKind(status.ActivityReplica) {
		t.Fatal("debía cancelar la réplica")
	}
	if ctx.Err() == nil {
		t.Fatal("la réplica no recibió la cancelación")
	}
	if l.idle() {
		t.Fatal("hasta que no suelta la sala no está libre")
	}
	stop()
	if !l.idle() {
		t.Fatal("tras soltarla la sala debía quedar libre")
	}

	verifyCtx, stopVerify, _ := l.enter(context.Background(), status.ActivityVerify)
	defer stopVerify()
	if l.cancelKind(status.ActivityReplica) || verifyCtx.Err() != nil {
		t.Fatal("una verificación no se cancela por la copia principal")
	}
}
