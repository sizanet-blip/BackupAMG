package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"backup-agent/internal/config"
	"backup-agent/internal/paths"
	"backup-agent/internal/report"
	"backup-agent/internal/restic"
	"backup-agent/internal/secrets"
	"backup-agent/internal/state"
	"backup-agent/internal/status"
	"backup-agent/internal/system"
)

// Acciones del panel sobre las copias adicionales. Todas son de administrador salvo el estado
// resumido, que va dentro de /api/v1/status y lo ve cualquiera.

// replicaListing es lo que muestra la pantalla Destinos.
func replicaListing() (any, error) {
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		return nil, err
	}
	st, _ := state.Load(paths.StatePath())
	views, _ := replicaViews(cfg, st, time.Now())
	byID := map[string]status.Replica{}
	for _, v := range views {
		byID[v.ID] = v
	}
	items := make([]map[string]any, 0, len(cfg.Replicas))
	for _, r := range cfg.Replicas {
		resolved := ""
		connected := false
		var free int64
		if located, locErr := system.Locate(r.Path, r.VolumeID); locErr == nil {
			resolved = located
			connected = dirExistsAt(located)
			if connected {
				free, _ = system.FreeBytes(located)
			}
		}
		view := byID[r.ID]
		item := map[string]any{
			"id": r.ID, "name": r.Name, "type": r.Type, "path": r.Path, "resolved": resolved,
			"volume_id": r.VolumeID, "enabled": r.Enabled, "when": r.When, "time": r.Time,
			"connected": connected, "free_bytes": free, "kind": system.DriveKind(r.Path),
			"state": view.State, "stale": view.Stale, "days_behind": view.DaysBehind,
			"last_success_at": view.LastSuccessAt, "message": view.Message, "running": view.Running,
		}
		if rs := st.Replicas[r.ID]; rs != nil {
			item["last_attempt_at"] = rs.LastAttempt
			item["interrupted"] = rs.Interrupted
		}
		items = append(items, item)
	}
	return map[string]any{"primary": destinationView(cfg), "replicas": items}, nil
}

type replicaForm struct {
	Name    *string `json:"name"`
	Path    *string `json:"path"`
	Enabled *bool   `json:"enabled"`
	When    *string `json:"when"`
	Time    *string `json:"time"`
}

var fieldPrefix = regexp.MustCompile(`^replicas\[\d+\]\.[a-z_]+: `)

// plainProblems quita de los avisos de validación el nombre técnico del campo.
func plainProblems(err error) ([]string, bool) {
	var invalid *config.ValidationError
	if !errors.As(err, &invalid) {
		return nil, false
	}
	out := make([]string, 0, len(invalid.Problems))
	for _, p := range invalid.Problems {
		p = fieldPrefix.ReplaceAllString(p, "")
		if p != "" {
			p = strings.ToUpper(p[:1]) + p[1:]
			if !strings.HasSuffix(p, ".") {
				p += "."
			}
		}
		out = append(out, p)
	}
	return out, true
}

func slugReplicaID(name string, taken map[string]bool) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	id := strings.Trim(b.String(), "-")
	if len(id) > 30 {
		id = strings.Trim(id[:30], "-")
	}
	if id == "" {
		id = "copia"
	}
	candidate := id
	for n := 2; taken[candidate]; n++ {
		candidate = fmt.Sprintf("%s-%d", id, n)
	}
	return candidate
}

// replicaTarget valida la carpeta y calcula el identificador de disco.
func replicaTarget(raw string) (path, volumeID string, problems []string) {
	path = filepath.Clean(strings.TrimSpace(raw))
	if strings.TrimSpace(raw) == "" || !(isDrivePath(path) || strings.HasPrefix(path, `\\`)) {
		return "", "", []string{`Elige una carpeta: un disco (por ejemplo F:\Copias) o una carpeta de red (\\nas\copias).`}
	}
	if isDrivePath(path) {
		switch system.DriveKind(path) {
		case "remote":
			return "", "", []string{"Esta letra es una unidad de red. El servicio no la ve: escribe \\\\servidor\\carpeta."}
		case "missing":
			return "", "", []string{"No se encuentra ese disco. Conéctalo y elige la carpeta otra vez."}
		}
		id, err := system.VolumeGUID(path)
		if err != nil {
			return "", "", []string{"No se encuentra ese disco. Conéctalo y elige la carpeta otra vez."}
		}
		volumeID = id
	}
	return path, volumeID, nil
}

// prepareReplicaRepo deja lista la carpeta de la copia adicional con la contraseña del principal.
func prepareReplicaRepo(cfg *config.Config, path string) []string {
	password, err := secrets.Open(cfg.Repository.PasswordEncrypted)
	if err != nil {
		return []string{"No se pudo usar la contraseña guardada, así que no se ha añadido."}
	}
	primary, err := system.Locate(cfg.Repository.Path, cfg.Repository.VolumeID)
	if err != nil || !resticRepo(primary) {
		return []string{"La copia principal no está disponible. Conéctala y vuelve a intentarlo."}
	}
	exe, err := restic.Executable()
	if err != nil {
		return []string{"No se encontró el programa de copias."}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if resticRepo(path) {
		ok, _, err := restic.PasswordOK(ctx, exe, path, password)
		if err != nil {
			return []string{"No se pudo comprobar la carpeta (" + err.Error() + ")."}
		}
		if !ok {
			return []string{"Esta carpeta ya tiene copias hechas con otra contraseña. Elige otra carpeta."}
		}
		return nil
	}
	if _, err := restic.InitReplica(ctx, restic.ReplicaInitRequest{Executable: exe, Repository: path, Source: primary, Password: password}); err != nil {
		return []string{"No se ha podido preparar la carpeta (" + err.Error() + ")."}
	}
	return nil
}

func addReplica(raw []byte) ([]string, error) {
	var form replicaForm
	if err := json.Unmarshal(raw, &form); err != nil {
		return []string{"El formulario no se entiende."}, nil
	}
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		return nil, err
	}
	path, volumeID, problems := replicaTarget(deref(form.Path))
	if len(problems) > 0 {
		return problems, nil
	}
	taken := map[string]bool{}
	for _, r := range cfg.Replicas {
		taken[r.ID] = true
	}
	name := strings.TrimSpace(deref(form.Name))
	if name == "" {
		name = "Copia adicional"
	}
	r := config.Replica{
		ID: slugReplicaID(name, taken), Name: name, Path: path, VolumeID: volumeID,
		Enabled: form.Enabled == nil || *form.Enabled, When: config.ReplicaAfterBackup,
	}
	if form.When != nil && *form.When != "" {
		r.When = *form.When
	}
	r.Time = deref(form.Time)
	if r.Path != "" && strings.HasPrefix(r.Path, `\\`) {
		r.Type = config.ReplicaUNC
	} else {
		r.Type = config.ReplicaLocal
	}
	list := append(append([]config.Replica{}, cfg.Replicas...), r)
	if err := config.ValidateReplicas(cfg, list); err != nil {
		if problems, ok := plainProblems(err); ok {
			return problems, nil
		}
		return nil, err
	}
	if problems := prepareReplicaRepo(cfg, path); len(problems) > 0 {
		return problems, nil
	}
	if err := config.SetReplicas(paths.ConfigPath(), list); err != nil {
		if problems, ok := plainProblems(err); ok {
			return problems, nil
		}
		return nil, err
	}
	touchReplicaState(r.ID, true)
	signalConfigReload()
	return nil, nil
}

func updateReplica(id string, raw []byte) ([]string, error) {
	var form replicaForm
	if err := json.Unmarshal(raw, &form); err != nil {
		return []string{"El formulario no se entiende."}, nil
	}
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		return nil, err
	}
	list := append([]config.Replica{}, cfg.Replicas...)
	idx := -1
	for i := range list {
		if list[i].ID == id {
			idx = i
		}
	}
	if idx < 0 {
		return []string{"Esa copia adicional ya no existe."}, nil
	}
	r := list[idx]
	pathChanged := false
	if form.Path != nil && !sameDestination(*form.Path, r.Path) {
		path, volumeID, problems := replicaTarget(*form.Path)
		if len(problems) > 0 {
			return problems, nil
		}
		r.Path, r.VolumeID, pathChanged = path, volumeID, true
		if strings.HasPrefix(path, `\\`) {
			r.Type = config.ReplicaUNC
		} else {
			r.Type = config.ReplicaLocal
		}
	}
	if form.Name != nil && strings.TrimSpace(*form.Name) != "" {
		r.Name = strings.TrimSpace(*form.Name)
	}
	if form.Enabled != nil {
		r.Enabled = *form.Enabled
	}
	if form.When != nil && *form.When != "" {
		r.When = *form.When
		r.Time = deref(form.Time)
	} else if form.Time != nil {
		r.Time = *form.Time
	}
	if r.When == config.ReplicaAfterBackup {
		r.Time = ""
	}
	list[idx] = r
	if err := config.ValidateReplicas(cfg, list); err != nil {
		if problems, ok := plainProblems(err); ok {
			return problems, nil
		}
		return nil, err
	}
	if pathChanged {
		if problems := prepareReplicaRepo(cfg, r.Path); len(problems) > 0 {
			return problems, nil
		}
	}
	if err := config.SetReplicas(paths.ConfigPath(), list); err != nil {
		if problems, ok := plainProblems(err); ok {
			return problems, nil
		}
		return nil, err
	}
	if pathChanged {
		// Es otro sitio: empieza de cero y se pone al día entero.
		forgetReplicaState(id)
		touchReplicaState(id, true)
	}
	signalConfigReload()
	return nil, nil
}

// removeReplica deja de copiar en esa carpeta. No borra nada de lo que ya hay allí.
func removeReplica(id string) error {
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		return err
	}
	var list []config.Replica
	found := false
	for _, r := range cfg.Replicas {
		if r.ID == id {
			found = true
			continue
		}
		list = append(list, r)
	}
	if !found {
		return errors.New("Esa copia adicional ya no existe.")
	}
	if err := config.SetReplicas(paths.ConfigPath(), list); err != nil {
		return err
	}
	if cur := replicaRunning.Load(); cur != nil && *cur == id {
		live.cancelKind(status.ActivityReplica)
	}
	forgetReplicaState(id)
	signalConfigReload()
	return nil
}

func testReplica(raw []byte) (any, error) {
	var form replicaForm
	if err := json.Unmarshal(raw, &form); err != nil {
		return map[string]any{"ok": false, "message": "El formulario no se entiende."}, nil
	}
	path, volumeID, problems := replicaTarget(deref(form.Path))
	if len(problems) > 0 {
		return map[string]any{"ok": false, "message": problems[0]}, nil
	}
	if cfg, err := config.Load(paths.ConfigPath()); err == nil {
		probe := append(append([]config.Replica{}, cfg.Replicas...), config.Replica{
			ID: "prueba", Name: "prueba", Type: config.ReplicaLocal, Path: path, VolumeID: volumeID, Enabled: true, When: config.ReplicaAfterBackup,
		})
		if verr := config.ValidateReplicas(cfg, probe); verr != nil {
			if problems, ok := plainProblems(verr); ok && len(problems) > 0 {
				return map[string]any{"ok": false, "message": problems[0]}, nil
			}
		}
	}
	return probeDestination(path), nil
}

// runReplicaNow actualiza ya una copia adicional (botón «Actualizar ahora»).
func runReplicaNow(id string) error {
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		return err
	}
	found := false
	for _, r := range cfg.Replicas {
		if r.ID == id {
			if !r.Enabled {
				return errors.New("Esa copia adicional está en pausa. Actívala para actualizarla.")
			}
			found = true
		}
	}
	if !found {
		return errors.New("Esa copia adicional ya no existe.")
	}
	live.mu.Lock()
	parent, logger := live.parent, live.logger
	live.mu.Unlock()
	if parent == nil || logger == nil {
		return errNotReady
	}
	if !live.idle() {
		return errBusy
	}
	go func() {
		runReplicas(parent, logger, []string{id}, report.TriggerManual)
		live.publish()
	}()
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// touchReplicaState crea (o reinicia) el estado de una copia adicional recién añadida.
func touchReplicaState(id string, reset bool) {
	stateMu.Lock()
	defer stateMu.Unlock()
	st, err := state.Load(paths.StatePath())
	if err != nil {
		return
	}
	if st.Replicas == nil {
		st.Replicas = map[string]*state.ReplicaState{}
	}
	if st.Replicas[id] == nil || reset {
		st.Replicas[id] = &state.ReplicaState{AddedAt: time.Now().Format(time.RFC3339)}
		_ = state.Save(paths.StatePath(), st)
	}
}

func forgetReplicaState(id string) {
	stateMu.Lock()
	defer stateMu.Unlock()
	st, err := state.Load(paths.StatePath())
	if err != nil || st.Replicas[id] == nil {
		return
	}
	delete(st.Replicas, id)
	if len(st.Replicas) == 0 {
		st.Replicas = nil
	}
	_ = state.Save(paths.StatePath(), st)
}
