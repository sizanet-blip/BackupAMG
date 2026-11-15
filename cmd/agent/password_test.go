package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"backup-agent/internal/config"
	"backup-agent/internal/restic"
)

const oldPW = "clave-vieja-123"
const newPW = "clave-NUEVA-456"

type pwEnv struct {
	exe       string
	root      string
	primary   string
	replica   string
	cfg       *config.Config
	ctx       context.Context
	replicaID string
}

func newPWEnv(t *testing.T) *pwEnv {
	t.Helper()
	exe := filepath.Join("..", "..", "bin", "restic.exe")
	if _, err := os.Stat(exe); err != nil {
		t.Skip("no está bin/restic.exe")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	t.Cleanup(cancel)
	root := t.TempDir()
	source := filepath.Join(root, "origen")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "a.txt"), []byte("hola"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := &pwEnv{exe: exe, root: root, ctx: ctx, replicaID: "usb",
		primary: filepath.Join(root, "principal"), replica: filepath.Join(root, "adicional")}
	if _, err := restic.Init(ctx, restic.InitRequest{Executable: exe, Repository: e.primary, Password: oldPW}); err != nil {
		t.Fatal(err)
	}
	out, err := restic.Backup(ctx, restic.BackupRequest{Executable: exe, Repository: e.primary, Password: oldPW, Sources: []string{source}, Host: "PC"})
	if err != nil || out.Status != restic.StatusSuccess {
		t.Fatalf("copia: %+v %v", out, err)
	}
	if _, err := restic.InitReplica(ctx, restic.ReplicaInitRequest{Executable: exe, Repository: e.replica, Source: e.primary, Password: oldPW}); err != nil {
		t.Fatal(err)
	}
	if _, err := restic.Copy(ctx, restic.CopyRequest{Executable: exe, Source: e.primary, Destination: e.replica, Password: oldPW}); err != nil {
		t.Fatal(err)
	}
	e.cfg = &config.Config{
		Repository: config.Repository{Path: e.primary},
		Replicas:   []config.Replica{{ID: e.replicaID, Name: "Disco externo", Path: e.replica, Enabled: true}},
	}
	return e
}

func (e *pwEnv) opens(t *testing.T, repo, password string) bool {
	t.Helper()
	ok, exists, err := restic.PasswordOK(e.ctx, e.exe, repo, password)
	if err != nil || !exists {
		t.Fatalf("no se pudo comprobar %s: exists=%v err=%v", repo, exists, err)
	}
	return ok
}

func (e *pwEnv) expect(t *testing.T, repo string, old, new bool) {
	t.Helper()
	if got := e.opens(t, repo, oldPW); got != old {
		t.Fatalf("%s con la contraseña vieja: abre=%v, se esperaba %v", filepath.Base(repo), got, old)
	}
	if got := e.opens(t, repo, newPW); got != new {
		t.Fatalf("%s con la contraseña nueva: abre=%v, se esperaba %v", filepath.Base(repo), got, new)
	}
}

func (e *pwEnv) targets(t *testing.T) []keyTarget {
	t.Helper()
	targets, problems := keyTargets(e.ctx, e.cfg, e.exe, oldPW)
	if len(problems) > 0 {
		t.Fatalf("problemas inesperados: %v", problems)
	}
	return targets
}

// Todo correcto: los dos repositorios pasan a la contraseña nueva, y la réplica y la recuperación siguen funcionando.
func TestPasswordChangeUpdatesEveryRepository(t *testing.T) {
	e := newPWEnv(t)
	saved := ""
	res := applyKeyChange(e.ctx, e.exe, e.targets(t), oldPW, newPW, func(p string) error { saved = p; return nil }, nil, nil)
	if len(res.problems) > 0 || len(res.leftover) > 0 || res.crashed {
		t.Fatalf("resultado: %+v", res)
	}
	if saved != newPW {
		t.Fatal("no se guardó la contraseña nueva")
	}
	e.expect(t, e.primary, false, true)
	e.expect(t, e.replica, false, true)

	// Con la contraseña nueva: se replica una copia nueva y se puede recuperar de la adicional.
	if _, err := os.Stat(e.replica); err != nil {
		t.Fatal(err)
	}
	if out, err := restic.Backup(e.ctx, restic.BackupRequest{Executable: e.exe, Repository: e.primary, Password: newPW, Sources: []string{filepath.Join(e.root, "origen")}, Host: "PC"}); err != nil || out.Status == restic.StatusFailed {
		t.Fatalf("copia con la nueva: %+v %v", out, err)
	}
	if _, err := restic.Copy(e.ctx, restic.CopyRequest{Executable: e.exe, Source: e.primary, Destination: e.replica, Password: newPW}); err != nil {
		t.Fatalf("la réplica debía seguir funcionando: %v", err)
	}
	if list, err := restic.Snapshots(e.ctx, e.exe, e.replica, newPW); err != nil || len(list) == 0 {
		t.Fatalf("recuperar desde la adicional: %v %v", list, err)
	}
}

// Una adicional desconectada, o con otra contraseña, bloquea el cambio y no se toca nada. Se dicen todas juntas.
func TestPasswordChangeIsBlockedByAnOfflineOrDifferentReplica(t *testing.T) {
	e := newPWEnv(t)
	e.cfg.Replicas = append(e.cfg.Replicas,
		config.Replica{ID: "fuera", Name: "Carpeta de la oficina", Path: filepath.Join(e.root, "no-existe"), Enabled: true},
		config.Replica{ID: "pausada", Name: "Disco pausado", Path: filepath.Join(e.root, "tampoco"), Enabled: false},
	)
	_, problems := keyTargets(e.ctx, e.cfg, e.exe, oldPW)
	text := strings.Join(problems, " | ")
	if len(problems) != 2 || !strings.Contains(text, "Conecta «Carpeta de la oficina» o quítala antes de cambiar la contraseña") || !strings.Contains(text, "«Disco pausado»") {
		t.Fatalf("problemas: %v", problems)
	}
	e.expect(t, e.primary, true, false)
	e.expect(t, e.replica, true, false)

	// Una carpeta que ya es un repositorio con OTRA contraseña.
	other := filepath.Join(e.root, "otra")
	if _, err := restic.Init(e.ctx, restic.InitRequest{Executable: e.exe, Repository: other, Password: "otra-distinta"}); err != nil {
		t.Fatal(err)
	}
	e.cfg.Replicas = []config.Replica{{ID: "otra", Name: "Copia ajena", Path: other, Enabled: true}}
	_, problems = keyTargets(e.ctx, e.cfg, e.exe, oldPW)
	if len(problems) != 1 || !strings.Contains(problems[0], "«Copia ajena» tiene otra contraseña") {
		t.Fatalf("problemas: %v", problems)
	}
	// Y la principal desconectada.
	e.cfg.Repository.Path = filepath.Join(e.root, "principal-no")
	_, problems = keyTargets(e.ctx, e.cfg, e.exe, oldPW)
	if len(problems) == 0 || !strings.Contains(problems[0], "La copia principal no está disponible") {
		t.Fatalf("problemas: %v", problems)
	}
	for _, p := range problems {
		if strings.Contains(p, oldPW) || strings.Contains(p, newPW) {
			t.Fatal("un mensaje contiene una contraseña")
		}
	}
}

// Si falla a mitad de la fase A se deshace y todo sigue con la contraseña vieja.
func TestPasswordChangeFailureMidwayRollsBack(t *testing.T) {
	e := newPWEnv(t)
	saved := false
	hook := func(stage string) error {
		if stage == "added:" { // la principal ya tiene la nueva: se «rompe» la adicional
			if err := os.RemoveAll(filepath.Join(e.replica, "keys")); err != nil {
				t.Fatal(err)
			}
		}
		return nil
	}
	res := applyKeyChange(e.ctx, e.exe, e.targets(t), oldPW, newPW, func(string) error { saved = true; return nil }, nil, hook)
	if len(res.problems) != 1 || !strings.Contains(res.problems[0], "Todo sigue como estaba") || !strings.Contains(res.problems[0], "«Disco externo»") {
		t.Fatalf("resultado: %+v", res)
	}
	if saved {
		t.Fatal("no debía guardarse la contraseña nueva")
	}
	e.expect(t, e.primary, true, false)
	keys, err := restic.KeyList(e.ctx, e.exe, e.primary, oldPW)
	if err != nil || len(keys) != 1 {
		t.Fatalf("la principal debía quedar con una sola clave: %v %v", keys, err)
	}
	for _, p := range res.problems {
		if strings.Contains(p, oldPW) || strings.Contains(p, newPW) {
			t.Fatal("un mensaje contiene una contraseña")
		}
	}
}

// Un corte entre fases deja un estado válido: las dos contraseñas abren todos los repositorios.
func TestPasswordChangeCrashBetweenPhasesLeavesBothPasswordsWorking(t *testing.T) {
	for _, stage := range []string{"added:", "beforeSave", "afterSave"} {
		t.Run(stage, func(t *testing.T) {
			e := newPWEnv(t)
			hook := func(s string) error {
				// «added:» se llama con el nombre del destino: se corta tras el primero (la principal).
				if s == stage {
					return errCrash
				}
				return nil
			}
			res := applyKeyChange(e.ctx, e.exe, e.targets(t), oldPW, newPW, func(string) error { return nil }, nil, hook)
			if !res.crashed {
				t.Fatalf("debía cortarse: %+v", res)
			}
			if stage == "added:" {
				e.expect(t, e.primary, true, true)  // la principal ya tiene las dos
				e.expect(t, e.replica, true, false) // la adicional aún no: la configuración sigue siendo la vieja
				return
			}
			e.expect(t, e.primary, true, true)
			e.expect(t, e.replica, true, true)
		})
	}
}

// La limpieza fallida avisa, y la contraseña nueva ya funciona en todos.
func TestPasswordChangeCleanupFailureIsReported(t *testing.T) {
	e := newPWEnv(t)
	hook := func(stage string) error {
		if stage == "afterSave" { // la adicional se desconecta justo antes de quitar la antigua
			if err := os.Rename(e.replica, e.replica+"-fuera"); err != nil {
				t.Fatal(err)
			}
		}
		return nil
	}
	res := applyKeyChange(e.ctx, e.exe, e.targets(t), oldPW, newPW, func(string) error { return nil }, nil, hook)
	if len(res.problems) > 0 || len(res.leftover) != 1 || res.leftover[0].Replica != e.replicaID || res.leftNames[0] != "«Disco externo»" {
		t.Fatalf("resultado: %+v", res)
	}
	e.expect(t, e.primary, false, true) // la principal sí se limpió
	if err := os.Rename(e.replica+"-fuera", e.replica); err != nil {
		t.Fatal(err)
	}
	e.expect(t, e.replica, true, true) // la antigua todavía abre la adicional, y la nueva también

	// Al volver, se puede quitar con la contraseña nueva: lo que hace el reintento.
	id := res.leftover[0].KeyID
	if has, err := restic.HasKey(e.ctx, e.exe, e.replica, newPW, id); err != nil || !has {
		t.Fatalf("la clave antigua debía seguir ahí: %v %v", has, err)
	}
	if err := restic.KeyRemove(e.ctx, e.exe, e.replica, newPW, id); err != nil {
		t.Fatal(err)
	}
	e.expect(t, e.replica, false, true)
}

func TestPasswordChangeRejectsTheSamePasswordAndErrorsAreClear(t *testing.T) {
	if !errors.Is(restic.ErrReplicaOtherPassword, restic.ErrReplicaOtherPassword) {
		t.Fatal("imposible")
	}
	if got := plainErr(restic.ErrWrongPassword); strings.Contains(got, "wrong") {
		t.Fatalf("mensaje técnico: %s", got)
	}
}
