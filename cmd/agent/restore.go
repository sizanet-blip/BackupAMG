package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"backup-agent/internal/config"
	"backup-agent/internal/paths"
	"backup-agent/internal/restic"
	"backup-agent/internal/restore"
	"backup-agent/internal/status"
)

type restoreJob struct {
	Snapshot  string
	Paths     []string
	Mode      string
	Target    string
	Overwrite bool
}

func listSnapshots() (any, error) {
	cfg, password, exe, err := openRuntime()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	items, err := restic.Snapshots(ctx, exe, cfg.Repository.Path, password)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []restic.SnapshotInfo{}
	}
	return items, nil
}

func listSnapshotFiles(id, dir string) (any, error) {
	if !validSnapshot(id) {
		return nil, fmt.Errorf("no se encontró esa copia")
	}
	cfg, password, exe, err := openRuntime()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if strings.TrimSpace(dir) == "" {
		items, err := restic.Snapshots(ctx, exe, cfg.Repository.Path, password)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if item.ID == id || strings.HasPrefix(item.ID, id) {
				nodes := make([]restic.Node, 0, len(item.Paths))
				for _, path := range item.Paths {
					nodes = append(nodes, restic.Node{Name: filepath.Base(path), Path: path, Type: "dir"})
				}
				return nodes, nil
			}
		}
		return nil, fmt.Errorf("no se encontró esa copia")
	}
	return restic.ListChildren(ctx, exe, cfg.Repository.Path, password, id, dir)
}

func findSnapshot(id, pattern string) (any, error) {
	if !validSnapshot(id) {
		return nil, fmt.Errorf("no se encontró esa copia")
	}
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || len(pattern) > 80 || strings.ContainsAny(pattern, "\r\n") {
		return nil, fmt.Errorf("escribe el nombre o parte del nombre")
	}
	cfg, password, exe, err := openRuntime()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return restic.Find(ctx, exe, cfg.Repository.Path, password, id, pattern)
}

func suggestedTarget() (any, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return map[string]string{"target": ""}, nil
	}
	desktop := filepath.Join(home, "Desktop")
	if info, err := os.Stat(desktop); err != nil || !info.IsDir() {
		alt := filepath.Join(home, "Escritorio")
		if info, err := os.Stat(alt); err == nil && info.IsDir() {
			desktop = alt
		}
	}
	return map[string]string{"target": filepath.Join(desktop, "Restaurado_"+time.Now().Format("2006-01-02"))}, nil
}

func startRestore(raw []byte) ([]string, error) {
	var body struct {
		Snapshot  string   `json:"snapshot"`
		Paths     []string `json:"paths"`
		Mode      string   `json:"mode"`
		Target    string   `json:"target"`
		Overwrite bool     `json:"overwrite"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return []string{"El formulario no se entiende."}, nil
	}
	if !validSnapshot(body.Snapshot) {
		return []string{"Elige una copia."}, nil
	}
	if len(body.Paths) == 0 || len(body.Paths) > 100 {
		return []string{"Elige al menos un fichero o una carpeta."}, nil
	}
	var chosen []string
	seen := map[string]struct{}{}
	for _, path := range body.Paths {
		path = filepath.Clean(strings.TrimSpace(path))
		if !isDrivePath(path) && !strings.HasPrefix(path, `\\`) {
			return []string{"La selección tiene una ruta que no es válida."}, nil
		}
		key := strings.ToLower(path)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		chosen = append(chosen, path)
	}
	job := restoreJob{Snapshot: body.Snapshot, Paths: chosen, Mode: body.Mode, Overwrite: body.Overwrite}
	if body.Mode == "original" {
		job.Target = "ubicación original"
	} else if body.Mode == "folder" {
		target := filepath.Clean(strings.TrimSpace(body.Target))
		if !isDrivePath(target) && !strings.HasPrefix(target, `\\`) {
			return []string{"Elige la carpeta donde recuperar los ficheros."}, nil
		}
		cfg, err := config.Load(paths.ConfigPath())
		if err != nil {
			return nil, err
		}
		if overlaps(target, cfg.Repository.Path) {
			return []string{"Esa carpeta está dentro del destino de las copias. Elige otra."}, nil
		}
		if err := os.MkdirAll(target, 0o755); err != nil {
			return []string{"No se pudo crear la carpeta de recuperación."}, nil
		}
		job.Target = target
		job.Overwrite = false
	} else {
		return []string{"Elige si recuperar en otra carpeta o en el sitio original."}, nil
	}
	return nil, live.startRestore(job)
}

func (l *liveJob) startRestore(job restoreJob) error {
	l.mu.Lock()
	parent := l.parent
	logger := l.logger
	busy := l.cancel != nil || l.starting
	l.mu.Unlock()
	if parent == nil || logger == nil {
		return errNotReady
	}
	if busy {
		return errBusy
	}
	l.mu.Lock()
	if l.cancel != nil || l.starting {
		l.mu.Unlock()
		return errBusy
	}
	l.starting = true
	l.restore = nil
	l.mu.Unlock()
	go func() {
		defer func() {
			l.mu.Lock()
			l.starting = false
			l.mu.Unlock()
			l.publish()
		}()
		l.runRestore(parent, logger, job)
	}()
	return nil
}

func (l *liveJob) runRestore(parent context.Context, logger interface {
	Info(string, ...any)
	Error(string, ...any)
}, job restoreJob) {
	ctx, stop, ok := l.enter(parent, status.ActivityRestore)
	if !ok {
		l.setRestore(status.RestoreResult{Status: "failed", Message: "Ya hay una tarea en curso.", Errors: []string{}})
		return
	}
	defer stop()
	cfg, password, exe, err := openRuntime()
	if err != nil {
		l.setRestore(status.RestoreResult{Status: "failed", Message: err.Error(), Errors: []string{err.Error()}})
		return
	}
	logger.Info("Recuperación iniciada", "snapshot", job.Snapshot, "mode", job.Mode, "count", len(job.Paths), "target", job.Target)
	onStatus := func(progress restic.Status) { l.note(progress) }
	if job.Mode == "original" {
		temp, err := os.MkdirTemp("", "ba-restore-")
		if err != nil {
			l.setRestore(status.RestoreResult{Status: "failed", Message: "No se pudo preparar la recuperación.", Errors: []string{}})
			return
		}
		defer os.RemoveAll(temp)
		if _, err := restic.Restore(ctx, exe, cfg.Repository.Path, password, job.Snapshot, temp, job.Paths, true, onStatus); err != nil {
			l.setRestore(status.RestoreResult{Status: "failed", Target: job.Target, Message: err.Error(), Errors: []string{err.Error()}})
			logger.Error("La recuperación no se ha completado", "error", err.Error())
			return
		}
		files, renamed, errs := restore.Place(temp, job.Overwrite)
		l.setRestore(outcome("ubicación original", files, renamed, 0, errs))
		logger.Info("Recuperación terminada", "files", files, "renamed", renamed)
		return
	}
	result, err := restic.Restore(ctx, exe, cfg.Repository.Path, password, job.Snapshot, job.Target, job.Paths, false, onStatus)
	if err != nil && result.Files == 0 {
		l.setRestore(status.RestoreResult{Status: "failed", Target: job.Target, Message: err.Error(), Errors: []string{err.Error()}})
		logger.Error("La recuperación no se ha completado", "error", err.Error())
		return
	}
	errs := result.Errors
	if err != nil {
		errs = append(errs, err.Error())
	}
	l.setRestore(outcome(job.Target, result.Files, 0, result.Skipped, errs))
	logger.Info("Recuperación terminada", "files", result.Files, "target", job.Target)
}

func (l *liveJob) setRestore(result status.RestoreResult) {
	if result.Errors == nil {
		result.Errors = []string{}
	}
	l.mu.Lock()
	l.restore = &result
	l.mu.Unlock()
	l.publish()
}

func outcome(target string, files, renamed, skipped int, errs []string) status.RestoreResult {
	if len(errs) > 20 {
		errs = errs[:20]
	}
	state := "success"
	if len(errs) > 0 && files == 0 {
		state = "failed"
	} else if len(errs) > 0 || renamed > 0 || skipped > 0 {
		state = "warning"
	}
	message := fmt.Sprintf("Se han recuperado %d ficheros.", files)
	if renamed > 0 {
		message += fmt.Sprintf(" %d ya existían y se han guardado con otro nombre.", renamed)
	}
	if skipped > 0 {
		message += fmt.Sprintf(" %d ya existían y no se han sustituido.", skipped)
	}
	if len(errs) > 0 {
		message += " Hay errores."
	}
	return status.RestoreResult{Status: state, Files: files, Renamed: renamed, Target: target, Message: message, Errors: errs}
}

func overlaps(target, repo string) bool {
	target = strings.ToLower(filepath.Clean(target))
	repo = strings.ToLower(filepath.Clean(repo))
	if target == repo {
		return true
	}
	sep := string(filepath.Separator)
	return strings.HasPrefix(target+sep, repo+sep) || strings.HasPrefix(repo+sep, target+sep)
}

func validSnapshot(id string) bool {
	if len(id) < 8 || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !unicode.Is(unicode.ASCII_Hex_Digit, r) {
			return false
		}
	}
	return true
}
