package restic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const staleLockAge = 30 * time.Minute

type fileLock struct {
	Time     time.Time `json:"time"`
	Hostname string    `json:"hostname"`
	PID      int       `json:"pid"`
}

// IsStale indica si un bloqueo de este equipo ya no corresponde a un restic en marcha.
// Los bloqueos de otro equipo no se tocan.
func IsStale(lock fileLock, hostname string, now time.Time, alive func(int) bool) bool {
	if !strings.EqualFold(strings.TrimSpace(lock.Hostname), strings.TrimSpace(hostname)) {
		return false
	}
	if now.Sub(lock.Time) >= staleLockAge {
		return true
	}
	if alive == nil {
		return false
	}
	return !alive(lock.PID)
}

// StaleLockIDs lee locks/ del repositorio y devuelve los identificadores obsoletos.
func StaleLockIDs(repo, hostname string, now time.Time, alive func(int) bool) ([]string, error) {
	dir := filepath.Join(repo, "locks")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var lock fileLock
		if json.Unmarshal(body, &lock) != nil {
			continue
		}
		if IsStale(lock, hostname, now, alive) {
			ids = append(ids, entry.Name())
		}
	}
	return ids, nil
}

// ReleaseStale borra los bloqueos de este equipo cuyo proceso ya no existe.
// Desde restic 0.19 el fichero está cifrado: se lee con «restic cat lock».
// No toca los bloqueos de otro equipo. Los ficheros temporales a medias no bloquean el repositorio.
func ReleaseStale(ctx context.Context, exe, repo, password, hostname string, now time.Time, alive func(int) bool) (int, error) {
	dir := filepath.Join(repo, "locks")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || strings.Contains(entry.Name(), "-tmp-") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		lock, ok, err := inspectLock(ctx, exe, repo, password, path, entry.Name())
		if err != nil {
			return removed, err
		}
		if !ok || !IsStale(lock, hostname, now, alive) {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return removed, fmt.Errorf("no se pudo quitar el bloqueo antiguo: %w", err)
		}
		removed++
	}
	return removed, nil
}

func inspectLock(ctx context.Context, exe, repo, password, path, name string) (fileLock, bool, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return fileLock{}, false, err
	}
	var lock fileLock
	if json.Unmarshal(body, &lock) == nil && lock.Hostname != "" {
		return lock, true, nil
	}
	code, stdout, stderr, runErr := run(ctx, exe, password, repo, []string{"--no-lock", "cat", "lock", name}, nil)
	if runErr != nil && code == 0 {
		return fileLock{}, false, runErr
	}
	if code != 0 {
		return fileLock{}, false, policyError("no se pudo leer el bloqueo del repositorio", code, stderr)
	}
	if json.Unmarshal([]byte(stdout), &lock) != nil || lock.Hostname == "" {
		return fileLock{}, false, nil
	}
	return lock, true, nil
}
