package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backup-agent/internal/config"
	"backup-agent/internal/paths"
	"backup-agent/internal/restic"
	"backup-agent/internal/secrets"
	"backup-agent/internal/system"
)

func loadEditable() (any, error) {
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		return nil, err
	}
	return cfg.View(), nil
}

func saveEditable(raw []byte) ([]string, error) {
	var edit config.Editable
	if err := json.Unmarshal(raw, &edit); err != nil {
		return []string{"El formulario no se entiende."}, nil
	}
	err := config.Update(paths.ConfigPath(), edit)
	var invalid *config.ValidationError
	if errors.As(err, &invalid) {
		return invalid.Problems, nil
	}
	if err != nil {
		return nil, err
	}
	signalConfigReload()
	return nil, nil
}

func listFolders(path string) (any, error) {
	dirs, err := system.ListDirs(path)
	if err != nil {
		return nil, err
	}
	if dirs == nil {
		dirs = []system.Dir{}
	}
	return dirs, nil
}

func folderShortcuts() any {
	items := system.Shortcuts()
	if items == nil {
		items = []system.Shortcut{}
	}
	return items
}

func loadDestination() (any, error) {
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		return nil, err
	}
	return destinationView(cfg), nil
}

func destinationView(cfg *config.Config) map[string]any {
	kind := "local"
	if strings.HasPrefix(strings.ReplaceAll(cfg.Repository.Path, "/", `\`), `\\`) {
		kind = "unc"
	}
	resolved := ""
	connected := false
	var free int64
	if located, err := system.Locate(cfg.Repository.Path, cfg.Repository.VolumeID); err == nil {
		resolved = located
		info, statErr := os.Stat(located)
		connected = statErr == nil && info.IsDir()
		if connected {
			free, _ = system.FreeBytes(located)
		}
	}
	return map[string]any{
		"path":       cfg.Repository.Path,
		"resolved":   resolved,
		"volume_id":  cfg.Repository.VolumeID,
		"kind":       kind,
		"connected":  connected,
		"free_bytes": free,
	}
}

func saveDestination(raw []byte) ([]string, error) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return []string{"El formulario no se entiende."}, nil
	}
	path := filepath.Clean(strings.TrimSpace(body.Path))
	volumeID := ""
	if isDrivePath(path) {
		if system.DriveKind(path) == "remote" {
			return []string{"Esta letra es una unidad de red. El servicio no la ve: escribe \\\\servidor\\carpeta."}, nil
		}
		id, err := system.VolumeGUID(path)
		if err != nil {
			return []string{"No se encuentra ese disco. Conéctalo y elige la carpeta otra vez."}, nil
		}
		volumeID = id
	}
	if problems := prepareNewDestination(path); len(problems) > 0 {
		return problems, nil
	}
	err := config.SetDestination(paths.ConfigPath(), path, volumeID)
	var invalid *config.ValidationError
	if errors.As(err, &invalid) {
		return invalid.Problems, nil
	}
	if err != nil {
		return nil, err
	}
	signalConfigReload()
	return nil, nil
}

// prepareNewDestination deja listo el destino nuevo antes de cambiar la configuración.
// Si la carpeta no tiene copias, crea el repositorio con la contraseña que ya se usa.
// Si ya las tiene, solo comprueba que esa contraseña las abre.
// Si algo falla devuelve el motivo y la configuración en uso no se toca.
// No borra ni mueve nada del destino anterior.
func prepareNewDestination(newPath string) []string {
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		return nil // sin configuración válida no hay nada en uso que proteger; SetDestination lo dirá
	}
	if sameDestination(cfg.Repository.Path, newPath) {
		return nil
	}
	password, err := secrets.Open(cfg.Repository.PasswordEncrypted)
	if err != nil {
		return []string{"No se pudo usar la contraseña guardada, así que el destino no se ha cambiado."}
	}
	if err := prepareRepository(newPath, password); err != nil {
		if strings.Contains(err.Error(), "no abre") {
			return []string{"En esa carpeta ya hay copias guardadas con otra contraseña, y la contraseña actual no las abre. El destino no se ha cambiado. Elige otra carpeta o usa la contraseña con la que se hicieron esas copias."}
		}
		return []string{"No se ha podido preparar la carpeta nueva (" + err.Error() + "). El destino no se ha cambiado."}
	}
	return nil
}

func sameDestination(a, b string) bool {
	return strings.EqualFold(filepath.Clean(strings.TrimSpace(a)), filepath.Clean(strings.TrimSpace(b)))
}

func testDestination(raw []byte) (any, error) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return map[string]any{"ok": false, "message": "El formulario no se entiende."}, nil
	}
	return probeDestination(strings.TrimSpace(body.Path)), nil
}

func probeDestination(path string) map[string]any {
	path = filepath.Clean(path)
	result := map[string]any{
		"ok": false, "accessible": false, "writable": false,
		"free_bytes": int64(0), "repository": false, "password_ok": nil,
		"resolved": path, "message": "",
	}
	if path == "" || path == "." || !(isDrivePath(path) || strings.HasPrefix(path, `\\`)) {
		result["message"] = `La ruta debe ser absoluta o UNC, por ejemplo E:\Copias o \\nas\copias.`
		return result
	}
	if isDrivePath(path) && system.DriveKind(path) == "remote" {
		result["message"] = "Esta letra es una unidad de red. El servicio no la ve: escribe \\\\servidor\\carpeta."
		return result
	}
	if isDrivePath(path) && system.DriveKind(path) == "missing" {
		result["message"] = "Destino no disponible: conecta el disco USB o comprueba la ruta."
		return result
	}
	dir, dirExists := reachableDir(path)
	if !dirExists {
		result["message"] = "Destino no disponible: conecta el disco USB o comprueba la ruta."
		return result
	}
	result["accessible"] = true
	if free, err := system.FreeBytes(dir); err == nil {
		result["free_bytes"] = free
	}
	target := path
	if !dirExistsAt(path) {
		target = dir
	}
	if writable(target) {
		result["writable"] = true
	}
	repo := dirExistsAt(path) && resticRepo(path)
	result["repository"] = repo
	if !repo {
		if result["writable"] == true {
			result["ok"] = true
			if dirExistsAt(path) {
				result["message"] = "Se puede escribir. Todavía no hay copias ahí; se preparará para guardarlas."
			} else {
				result["message"] = "Se puede escribir. La carpeta se creará y se preparará para guardar las copias."
			}
		} else {
			result["message"] = "No se puede escribir en esa carpeta."
		}
		return result
	}
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		result["message"] = "Ahí ya hay copias. En el paso de la contraseña usa la de esas copias."
		result["ok"] = result["writable"] == true
		return result
	}
	password, err := secrets.Open(cfg.Repository.PasswordEncrypted)
	if err != nil {
		result["message"] = "No se pudo usar la contraseña guardada."
		return result
	}
	exe, err := restic.Executable()
	if err != nil {
		result["message"] = "No se encontró el programa de copias."
		return result
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ok, _, checkErr := restic.PasswordOK(ctx, exe, path, password)
	result["password_ok"] = ok
	if checkErr != nil {
		result["message"] = "No se pudo comprobar la contraseña."
		return result
	}
	if !ok {
		result["message"] = "La contraseña guardada no abre este destino."
		return result
	}
	if result["writable"] != true {
		result["message"] = "La contraseña es correcta, pero no se puede escribir."
		return result
	}
	result["ok"] = true
	result["message"] = "El destino responde y la contraseña es correcta."
	return result
}

func changePassword(raw []byte) ([]string, error) {
	var body struct {
		Password     string `json:"password"`
		Confirm      string `json:"confirm"`
		Acknowledged bool   `json:"acknowledged"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return []string{"El formulario no se entiende."}, nil
	}
	var problems []string
	if strings.TrimSpace(body.Password) == "" {
		problems = append(problems, "La contraseña está vacía.")
	}
	if body.Password != body.Confirm {
		problems = append(problems, "Las contraseñas no coinciden.")
	}
	if !body.Acknowledged {
		problems = append(problems, "Marca que has guardado la contraseña en un lugar seguro.")
	}
	if len(problems) > 0 {
		return problems, nil
	}
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		return nil, err
	}
	resolved, err := system.Locate(cfg.Repository.Path, cfg.Repository.VolumeID)
	if err != nil {
		return []string{"Conecta el destino antes de cambiar la contraseña."}, nil
	}
	if !reachable(resolved) {
		return []string{"Conecta el destino antes de cambiar la contraseña."}, nil
	}
	old, err := secrets.Open(cfg.Repository.PasswordEncrypted)
	if err != nil {
		return nil, err
	}
	if dirExistsAt(resolved) && resticRepo(resolved) {
		exe, err := restic.Executable()
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := restic.ChangePassword(ctx, exe, resolved, old, body.Password); err != nil {
			return nil, err
		}
		if saveErr := saveNewPassword(body.Password); saveErr != nil {
			_ = restic.ChangePassword(ctx, exe, resolved, body.Password, old)
			return nil, saveErr
		}
		return nil, nil
	}
	return nil, saveNewPassword(body.Password)
}

func saveNewPassword(password string) error {
	encoded, err := secrets.Protect(password)
	if err != nil {
		return err
	}
	if err := config.SetPasswordEncrypted(paths.ConfigPath(), encoded); err != nil {
		return err
	}
	signalConfigReload()
	return nil
}

func isDrivePath(path string) bool {
	return len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/')
}

func resticRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, "config"))
	return err == nil && !info.IsDir()
}

func dirExistsAt(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func reachable(path string) bool {
	_, ok := reachableDir(path)
	return ok
}

func reachableDir(path string) (string, bool) {
	current := filepath.Clean(path)
	for {
		info, err := os.Stat(current)
		if err == nil && info.IsDir() {
			return current, true
		}
		if err == nil {
			return "", false
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

func writable(dir string) bool {
	file, err := os.CreateTemp(dir, ".ba-probe-*")
	if err != nil {
		return false
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return true
}

func estimateSources(paths []string) (int64, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	total, complete := system.EstimateBytes(ctx, paths)
	return total, complete, nil
}
