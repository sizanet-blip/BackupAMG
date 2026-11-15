package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"backup-agent/internal/config"
	"backup-agent/internal/logging"
	"backup-agent/internal/panel"
	"backup-agent/internal/paths"
	"backup-agent/internal/restic"
	"backup-agent/internal/secrets"
	"backup-agent/internal/state"
	"backup-agent/internal/status"
	"backup-agent/internal/system"
)

// Cambiar la contraseña toca TODOS los repositorios (la copia principal y cada copia adicional)
// y nunca deja repositorios con contraseñas distintas en uso:
//
//	0. Todos tienen que estar conectados y abrirse con la contraseña actual. Si no, no se hace nada.
//	A. En cada repositorio se AÑADE la contraseña nueva (restic key add): acepta las dos.
//	   Se comprueba que la nueva abre todos. Si algo falla, se quitan las nuevas y no cambia nada.
//	C. Se guarda la nueva en la configuración (cifrada con DPAPI). Desde aquí es la que se usa.
//	B. Se quita la antigua de cada repositorio. Si alguna falla, la nueva ya está en uso y la
//	   antigua queda anotada para quitarla después.
//
// Un corte en cualquier punto deja un estado válido: o la configuración antigua o la nueva,
// y las dos abren todos los repositorios hasta que termina la fase B.

var passwordChanging atomic.Bool

// waitPasswordChange hace esperar a una copia que llega mientras se cambia la contraseña:
// dura segundos y así la copia programada no se pierde.
func waitPasswordChange(ctx context.Context) {
	for i := 0; i < 1200 && passwordChanging.Load() && ctx.Err() == nil; i++ {
		time.Sleep(100 * time.Millisecond)
	}
}

type keyTarget struct {
	replicaID string // vacío: copia principal
	name      string
	repo      string // ruta ya resuelta
	oldKey    string
	newKey    string
}

const keyOpTimeout = 2 * time.Minute

// changePasswordEverywhere devuelve los problemas que impiden el cambio, o un *panel.Notice si
// salió bien pero queda algo por limpiar.
func changePasswordEverywhere(newPassword string) ([]string, error) {
	live.mu.Lock()
	parent, logger := live.parent, live.logger
	live.mu.Unlock()
	if parent == nil {
		parent = context.Background()
	}
	ctx, stop, ok := live.enter(parent, status.ActivityPassword)
	if !ok {
		return []string{"Hay una copia o una recuperación en marcha. Espera a que termine y vuelve a intentarlo."}, nil
	}
	defer stop()
	passwordChanging.Store(true)
	defer passwordChanging.Store(false)

	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		return nil, err
	}
	oldPassword, err := secrets.Open(cfg.Repository.PasswordEncrypted)
	if err != nil {
		return nil, err
	}
	if newPassword == oldPassword {
		return []string{"La contraseña nueva es igual que la actual."}, nil
	}
	exe, err := restic.Executable()
	if err != nil {
		return nil, err
	}
	// 0. Todo conectado y con la contraseña actual.
	targets, problems := keyTargets(ctx, cfg, exe, oldPassword)
	if len(problems) > 0 {
		return problems, nil
	}

	res := applyKeyChange(ctx, exe, targets, oldPassword, newPassword, saveNewPassword, logger, nil)
	if len(res.problems) > 0 {
		return res.problems, nil
	}
	if len(res.leftover) > 0 {
		recordPendingRemovals(res.leftover)
		return nil, &panel.Notice{Message: "La contraseña nueva ya se usa en todos los destinos. La antigua todavía abre " + strings.Join(res.leftNames, ", ") + ": se quitará en cuanto se pueda."}
	}
	return nil, nil
}

type keyChangeResult struct {
	problems  []string
	leftover  []state.KeyRemoval
	leftNames []string
	crashed   bool
}

// errCrash lo usan las pruebas para simular un corte entre fases.
var errCrash = errors.New("corte simulado")

// applyKeyChange hace las fases A, C y B sobre repositorios ya comprobados.
// hook, si no es nil, se llama entre fases; si devuelve errCrash se sale sin deshacer nada.
func applyKeyChange(ctx context.Context, exe string, targets []keyTarget, oldPassword, newPassword string, save func(string) error, logger *logging.Logger, hook func(stage string) error) keyChangeResult {
	stage := func(name string) bool { return hook != nil && errors.Is(hook(name), errCrash) }

	// A. Añadir la nueva en cada repositorio y comprobar que abre.
	for i := range targets {
		t := &targets[i]
		opCtx, cancel := context.WithTimeout(ctx, keyOpTimeout)
		id, addErr := restic.KeyAdd(opCtx, exe, t.repo, oldPassword, newPassword)
		cancel()
		if addErr != nil {
			rollbackKeys(exe, oldPassword, targets, logger)
			return keyChangeResult{problems: []string{fmt.Sprintf("No se ha cambiado la contraseña: no se pudo actualizar %s (%s). Todo sigue como estaba.", t.label(), plainErr(addErr))}}
		}
		t.newKey = id
		if stage("added:" + t.name) {
			return keyChangeResult{crashed: true}
		}
	}
	for _, t := range targets {
		opCtx, cancel := context.WithTimeout(ctx, keyOpTimeout)
		opens, _, checkErr := restic.PasswordOK(opCtx, exe, t.repo, newPassword)
		cancel()
		if checkErr != nil || !opens {
			rollbackKeys(exe, oldPassword, targets, logger)
			return keyChangeResult{problems: []string{fmt.Sprintf("No se ha cambiado la contraseña: la nueva no abre %s. Todo sigue como estaba.", t.label())}}
		}
	}
	if stage("beforeSave") {
		return keyChangeResult{crashed: true}
	}

	// C. Desde aquí, la contraseña en uso es la nueva.
	if err := save(newPassword); err != nil {
		rollbackKeys(exe, oldPassword, targets, logger)
		return keyChangeResult{problems: []string{"No se ha cambiado la contraseña: no se pudo guardar. Todo sigue como estaba."}}
	}
	if logger != nil {
		logger.Info("Contraseña cambiada en todos los destinos", "repositories", len(targets))
	}
	if stage("afterSave") {
		return keyChangeResult{crashed: true}
	}

	// B. Quitar la antigua. Si falla alguna, se anota y se reintenta.
	var res keyChangeResult
	for _, t := range targets {
		opCtx, cancel := context.WithTimeout(context.Background(), keyOpTimeout)
		rmErr := restic.KeyRemove(opCtx, exe, t.repo, newPassword, t.oldKey)
		cancel()
		if rmErr != nil {
			res.leftover = append(res.leftover, state.KeyRemoval{Replica: t.replicaID, KeyID: t.oldKey})
			res.leftNames = append(res.leftNames, t.label())
			if logger != nil {
				logger.Warn("No se pudo quitar la contraseña antigua", "repository", t.name, "error", plainErr(rmErr))
			}
		}
	}
	return res
}

func (t keyTarget) label() string {
	if t.replicaID == "" {
		return "la copia principal"
	}
	return "«" + t.name + "»"
}

// keyTargets prepara la lista de repositorios y comprueba, para todos a la vez, que están
// conectados y se abren con la contraseña actual. Devuelve todos los problemas juntos.
func keyTargets(ctx context.Context, cfg *config.Config, exe, password string) ([]keyTarget, []string) {
	var targets []keyTarget
	var problems []string
	add := func(replicaID, name, path, volumeID string) {
		t := keyTarget{replicaID: replicaID, name: name}
		located, err := system.Locate(path, volumeID)
		if err != nil || !dirExistsAt(located) || !resticRepo(located) {
			if replicaID == "" {
				problems = append(problems, "La copia principal no está disponible. Conéctala antes de cambiar la contraseña.")
			} else {
				problems = append(problems, "Conecta «"+name+"» o quítala antes de cambiar la contraseña.")
			}
			return
		}
		t.repo = located
		opCtx, cancel := context.WithTimeout(ctx, keyOpTimeout)
		defer cancel()
		id, err := restic.CurrentKeyID(opCtx, exe, located, password)
		if err != nil {
			if errors.Is(err, restic.ErrWrongPassword) {
				if replicaID == "" {
					problems = append(problems, "La copia principal no se abre con la contraseña guardada.")
				} else {
					problems = append(problems, "«"+name+"» tiene otra contraseña que la copia principal y no se puede cambiar así. Quítala y vuelve a añadirla.")
				}
			} else {
				problems = append(problems, "No se pudo comprobar "+t.label()+": "+plainErr(err)+".")
			}
			return
		}
		t.oldKey = id
		targets = append(targets, t)
	}
	add("", "", cfg.Repository.Path, cfg.Repository.VolumeID)
	for _, r := range cfg.Replicas {
		add(r.ID, r.Name, r.Path, r.VolumeID)
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return targets, nil
}

// rollbackKeys quita las contraseñas nuevas ya añadidas. Se hace con la contraseña antigua, que
// sigue valiendo en todos. Si algo falla aquí no se pierde nada: quedaría una clave de más.
func rollbackKeys(exe, oldPassword string, targets []keyTarget, logger *logging.Logger) {
	for _, t := range targets {
		if t.newKey == "" {
			continue
		}
		opCtx, cancel := context.WithTimeout(context.Background(), keyOpTimeout)
		err := restic.KeyRemove(opCtx, exe, t.repo, oldPassword, t.newKey)
		cancel()
		if err != nil && logger != nil {
			logger.Warn("No se pudo deshacer la contraseña nueva", "repository", t.name, "error", plainErr(err))
		}
	}
}

func plainErr(err error) string {
	msg := restic.PlainFailure(err.Error())
	return strings.TrimSuffix(strings.TrimSpace(msg), ".")
}

func recordPendingRemovals(list []state.KeyRemoval) {
	stateMu.Lock()
	defer stateMu.Unlock()
	st, err := state.Load(paths.StatePath())
	if err != nil {
		return
	}
	st.PendingKeyRemovals = append(st.PendingKeyRemovals, list...)
	_ = state.Save(paths.StatePath(), st)
}

// retryKeyCleanup quita las contraseñas antiguas que quedaron pendientes cuando el destino
// vuelve a estar disponible.
func retryKeyCleanup(ctx context.Context, logger *logging.Logger) {
	st, err := state.Load(paths.StatePath())
	if err != nil || len(st.PendingKeyRemovals) == 0 {
		return
	}
	ctx, stop, ok := live.enter(ctx, status.ActivityPassword)
	if !ok {
		return
	}
	defer stop()
	cfg, password, exe, err := openRuntime()
	if err != nil {
		return
	}
	var remaining []state.KeyRemoval
	for _, p := range st.PendingKeyRemovals {
		path, volume, exists := cfg.Repository.Path, cfg.Repository.VolumeID, true
		if p.Replica != "" {
			exists = false
			for _, r := range cfg.Replicas {
				if r.ID == p.Replica {
					path, volume, exists = r.Path, r.VolumeID, true
				}
			}
		}
		if !exists {
			continue // la copia adicional ya no existe: nada que quitar
		}
		located, locErr := system.Locate(path, volume)
		if locErr != nil || !dirExistsAt(located) || !resticRepo(located) {
			remaining = append(remaining, p)
			continue
		}
		opCtx, cancel := context.WithTimeout(ctx, keyOpTimeout)
		present, hasErr := restic.HasKey(opCtx, exe, located, password, p.KeyID)
		if hasErr == nil && present {
			hasErr = restic.KeyRemove(opCtx, exe, located, password, p.KeyID)
		}
		cancel()
		if hasErr != nil {
			remaining = append(remaining, p)
			continue
		}
		if logger != nil {
			logger.Info("Quitada una contraseña antigua que había quedado pendiente")
		}
	}
	if len(remaining) == len(st.PendingKeyRemovals) {
		return
	}
	stateMu.Lock()
	defer stateMu.Unlock()
	if fresh, err := state.Load(paths.StatePath()); err == nil {
		fresh.PendingKeyRemovals = remaining
		_ = state.Save(paths.StatePath(), fresh)
	}
}
