package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"backup-agent/internal/config"
	"backup-agent/internal/logging"
	"backup-agent/internal/paths"
	"backup-agent/internal/report"
	"backup-agent/internal/state"
)

func TestAutomaticVerificationWaitsForTheInterval(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BACKUP_AGENT_DATA_DIR", dir)
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Verify: config.Verify{EveryDays: 7}}
	now := time.Date(2026, 11, 13, 21, 0, 0, 0, time.Local)

	due, err := verificationDue(cfg, now)
	if err != nil || !due {
		t.Fatalf("sin verificación previa debe tocar: %v %v", due, err)
	}

	saveVerify(t, now.AddDate(0, 0, -6))
	due, err = verificationDue(cfg, now)
	if err != nil || due {
		t.Fatalf("a los 6 días no toca: %v %v", due, err)
	}

	saveVerify(t, now.AddDate(0, 0, -7))
	due, err = verificationDue(cfg, now)
	if err != nil || !due {
		t.Fatalf("a los 7 días toca: %v %v", due, err)
	}
}

func saveVerify(t *testing.T, at time.Time) {
	t.Helper()
	err := state.Save(paths.StatePath(), state.State{
		SchemaVersion: 1,
		LastVerify: &state.LastVerify{
			FinishedAt: at.Format(time.RFC3339),
			Status:     "success",
			Report:     "anterior.json",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

// verificationFixture prepara una configuración válida con el destino presente y cuenta
// las veces que se lanza la verificación (sin ejecutar restic).
func verificationFixture(t *testing.T, repositoryPresent bool) (*logging.Logger, *[]string) {
	t.Helper()
	oldPath, _ := destinationFixture(t, "clave-de-prueba")
	if repositoryPresent {
		// Basta con que exista el fichero de configuración del repositorio: restic no se ejecuta.
		if err := os.MkdirAll(oldPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(oldPath, "config"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	logger, err := logging.New(logging.Options{LogFile: paths.LogFile(), Console: false})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	var launched []string
	previous := verifyRunner
	verifyRunner = func(ctx context.Context, _ *slog.Logger, cfg *config.Config, password, exe, trigger string) error {
		launched = append(launched, trigger)
		return nil
	}
	t.Cleanup(func() { verifyRunner = previous })
	return logger, &launched
}

func TestPeriodicVerificationIsLaunchedByItselfWhenItIsTime(t *testing.T) {
	logger, launched := verificationFixture(t, true)

	// Nunca se ha verificado: toca.
	if !verifyIfDue(context.Background(), logger) || len(*launched) != 1 || (*launched)[0] != report.TriggerScheduled {
		t.Fatalf("sin verificación previa debería lanzarse una vez: %v", *launched)
	}
	// Verificada ayer: no toca todavía (verify.every_days por defecto: 7).
	saveVerify(t, time.Now().AddDate(0, 0, -1))
	if verifyIfDue(context.Background(), logger) || len(*launched) != 1 {
		t.Fatalf("a un día de la última no debería lanzarse: %v", *launched)
	}
	// Verificada hace 8 días: toca.
	saveVerify(t, time.Now().AddDate(0, 0, -8))
	if !verifyIfDue(context.Background(), logger) || len(*launched) != 2 {
		t.Fatalf("a los 8 días debería lanzarse: %v", *launched)
	}
}

func TestPeriodicVerificationWaitsWhileTheDestinationIsMissing(t *testing.T) {
	logger, launched := verificationFixture(t, false)
	if verifyIfDue(context.Background(), logger) || len(*launched) != 0 {
		t.Fatalf("sin destino no debe intentarse ni dejar una verificación fallida: %v", *launched)
	}
}

func TestVerificationFollowsOnlyASuccessfulBackup(t *testing.T) {
	logger, launched := verificationFixture(t, true)
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, failed := range []int{1, -1} {
		afterBackup(context.Background(), logger, cfg, failed)
	}
	if len(*launched) != 0 {
		t.Fatalf("tras una copia fallida o cancelada no se verifica: %v", *launched)
	}
	afterBackup(context.Background(), logger, cfg, 0) // correcta
	if len(*launched) != 1 {
		t.Fatalf("tras una copia correcta toca verificar: %v", *launched)
	}
	saveVerify(t, time.Now().AddDate(0, 0, -30))
	afterBackup(context.Background(), logger, cfg, 3) // con avisos
	if len(*launched) != 2 {
		t.Fatalf("tras una copia con avisos también: %v", *launched)
	}
}
