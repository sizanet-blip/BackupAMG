package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"backup-agent/internal/status"
)

func fastPasswordWait(t *testing.T, max time.Duration) {
	t.Helper()
	oldMax, oldStep := passwordWaitMax, passwordWaitStep
	passwordWaitMax, passwordWaitStep = max, 5*time.Millisecond
	t.Cleanup(func() { passwordWaitMax, passwordWaitStep = oldMax, oldStep })
}

// simulatePasswordChange ocupa la sala como lo hace el cambio de contraseña y la suelta pasado un rato.
func simulatePasswordChange(t *testing.T, duration time.Duration) (done chan struct{}) {
	t.Helper()
	passwordChanging.Store(true) // primero la señal y luego la sala, como el código real
	_, stop, ok := live.enter(context.Background(), status.ActivityPassword)
	if !ok {
		passwordChanging.Store(false)
		t.Fatal("la sala debía estar libre")
	}
	done = make(chan struct{})
	go func() {
		time.Sleep(duration)
		stop()
		passwordChanging.Store(false)
		close(done)
	}()
	return done
}

// Una copia que llega mientras dura el cambio espera a que termine y se hace; no se descarta.
func TestABackupArrivingDuringAPasswordChangeWaitsAndRuns(t *testing.T) {
	fastPasswordWait(t, 30*time.Minute)
	done := simulatePasswordChange(t, 600*time.Millisecond)
	started := time.Now()
	ctx, stop, ok := enterBackupRoom(context.Background())
	if !ok {
		t.Fatal("la copia se ha descartado en vez de esperar")
	}
	defer stop()
	<-done
	if time.Since(started) < 500*time.Millisecond {
		t.Fatalf("la copia no esperó al cambio: %v", time.Since(started))
	}
	if ctx.Err() != nil {
		t.Fatal("el contexto de la copia está cancelado")
	}
	if live.idle() {
		t.Fatal("la copia debía tener la sala")
	}
}

// El caso que se perdía antes: el cambio dura más que el viejo tope de 2 minutos. Aquí, a escala:
// el tope es mucho mayor que la duración del cambio, y la copia espera todo el tiempo necesario.
func TestABackupWaitsLongerThanTheOldTwoMinuteCap(t *testing.T) {
	fastPasswordWait(t, 30*time.Minute)
	if passwordWaitMax < 20*time.Minute {
		t.Fatal("el tope de espera debe ser muy largo")
	}
	done := simulatePasswordChange(t, 1500*time.Millisecond)
	if _, stop, ok := enterBackupRoom(context.Background()); !ok {
		t.Fatal("la copia se ha descartado")
	} else {
		stop()
	}
	<-done
}

// La señal se pone antes de pedir la sala: una copia que llega entre las dos cosas también espera.
func TestABackupArrivingBetweenTheSignalAndTheRoomWaits(t *testing.T) {
	fastPasswordWait(t, 30*time.Minute)
	passwordChanging.Store(true)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(200 * time.Millisecond)
		_, stop, ok := live.enter(context.Background(), status.ActivityPassword)
		if !ok {
			t.Error("la sala debía estar libre")
			passwordChanging.Store(false)
			return
		}
		time.Sleep(200 * time.Millisecond)
		stop()
		passwordChanging.Store(false)
	}()
	_, stop, ok := enterBackupRoom(context.Background())
	if !ok {
		t.Fatal("la copia se ha descartado")
	}
	stop()
	wg.Wait()
}

// Si se cancela mientras espera, sale sin colgarse. Y sin cambio de contraseña no espera nada.
func TestWaitingForAPasswordChangeCanBeCancelledAndIsFreeOtherwise(t *testing.T) {
	fastPasswordWait(t, 30*time.Minute)
	if !waitPasswordChange(context.Background()) {
		t.Fatal("sin cambio en marcha no debe esperar")
	}
	passwordChanging.Store(true)
	defer passwordChanging.Store(false)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	if waitPasswordChange(ctx) {
		t.Fatal("debía salir por cancelación")
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("tardó demasiado en salir")
	}
	// Si la sala la tiene otra tarea (no un cambio de contraseña), no se reintenta sin fin.
	passwordChanging.Store(false)
	_, release, ok := live.enter(context.Background(), status.ActivityVerify)
	if !ok {
		t.Fatal("la sala debía estar libre")
	}
	defer release()
	if _, _, ok := enterBackupRoom(context.Background()); ok {
		t.Fatal("con otra tarea en marcha la copia no debe entrar")
	}
}
