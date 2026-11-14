package restic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"backup-agent/internal/restore"
)

// SnapshotInfo es una copia guardada en el repositorio.
type SnapshotInfo struct {
	ID       string   `json:"id"`
	ShortID  string   `json:"short_id"`
	Time     string   `json:"time"`
	Hostname string   `json:"hostname"`
	Paths    []string `json:"paths"`
}

// Node es un fichero o una carpeta dentro de una copia.
type Node struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Type  string `json:"type"`
	Size  int64  `json:"size"`
	Mtime string `json:"mtime,omitempty"`
}

// RestoreResult cuenta lo que se ha extraído.
type RestoreResult struct {
	Files   int
	Skipped int
	Errors  []string
}

// Snapshots lista las copias del repositorio.
func Snapshots(ctx context.Context, exe, repo, password string) ([]SnapshotInfo, error) {
	code, stdout, stderr, err := run(ctx, exe, password, repo, []string{"snapshots", "--json"}, nil)
	if err != nil && code == 0 {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("%s", restoreMessage(code, stderr))
	}
	var items []SnapshotInfo
	if strings.TrimSpace(stdout) == "" {
		return []SnapshotInfo{}, nil
	}
	if err := json.Unmarshal([]byte(stdout), &items); err != nil {
		return nil, fmt.Errorf("no se pudo leer la lista de copias")
	}
	if items == nil {
		items = []SnapshotInfo{}
	}
	return items, nil
}

// ListChildren devuelve el contenido inmediato de dir dentro de una copia.
func ListChildren(ctx context.Context, exe, repo, password, snapshot, dir string) ([]Node, error) {
	args := []string{"ls", snapshot, "--json"}
	if dir != "" {
		args = append(args, restore.ToRestic(dir))
	}
	nodes, err := readNodes(ctx, exe, repo, password, args)
	if err != nil {
		return nil, err
	}
	parent := restore.ToWindows(dir)
	var children []Node
	for _, node := range nodes {
		if node.Path == parent || filepathDir(node.Path) != parent {
			continue
		}
		children = append(children, node)
	}
	if children == nil {
		children = []Node{}
	}
	return children, nil
}

// Find busca por nombre dentro de una copia. Como mucho devuelve 200 coincidencias.
func Find(ctx context.Context, exe, repo, password, snapshot, pattern string) ([]Node, error) {
	nodes, err := readNodes(ctx, exe, repo, password, []string{"find", "--json", "--snapshot", snapshot, pattern})
	if err != nil {
		return nil, err
	}
	if len(nodes) > 200 {
		nodes = nodes[:200]
	}
	if nodes == nil {
		nodes = []Node{}
	}
	return nodes, nil
}

// Restore extrae las rutas indicadas en target.
func Restore(ctx context.Context, exe, repo, password, snapshot, target string, paths []string, overwrite bool, onStatus func(Status)) (RestoreResult, error) {
	args := []string{"restore", snapshot, "--json", "--target", target}
	if overwrite {
		args = append(args, "--overwrite", "always")
	} else {
		args = append(args, "--overwrite", "never")
	}
	for _, path := range paths {
		resticPath := restore.ToRestic(path)
		args = append(args, "--include", resticPath, "--include", resticPath+"/**")
	}
	code, stdout, stderr, err := run(ctx, exe, password, repo, args, func(line Line) {
		if line.Status != nil && onStatus != nil {
			onStatus(*line.Status)
		}
	})
	if err != nil && code == 0 {
		return RestoreResult{}, err
	}
	result := parseRestore(stdout)
	if code != 0 && ctx.Err() != nil {
		return result, fmt.Errorf("la recuperación se ha cancelado")
	}
	if code != 0 {
		if len(result.Errors) == 0 && strings.TrimSpace(stderr) != "" {
			result.Errors = append(result.Errors, restoreMessage(code, stderr))
		}
		if result.Files == 0 {
			return result, fmt.Errorf("%s", restoreMessage(code, stderr))
		}
	}
	return result, nil
}

func readNodes(ctx context.Context, exe, repo, password string, args []string) ([]Node, error) {
	code, stdout, stderr, err := run(ctx, exe, password, repo, args, nil)
	if err != nil && code == 0 {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("%s", restoreMessage(code, stderr))
	}
	var nodes []Node
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nodes = append(nodes, nodesFrom(line)...)
	}
	return nodes, nil
}

func nodesFrom(line string) []Node {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "[") {
		var rows []json.RawMessage
		if json.Unmarshal([]byte(line), &rows) != nil {
			return nil
		}
		var nodes []Node
		for _, row := range rows {
			nodes = append(nodes, nodesFrom(string(row))...)
		}
		return nodes
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal([]byte(line), &raw) != nil {
		return nil
	}
	if matches, ok := raw["matches"]; ok {
		var found []struct {
			Path  string `json:"path"`
			Type  string `json:"type"`
			Size  int64  `json:"size"`
			Name  string `json:"name"`
			Mtime string `json:"mtime"`
		}
		if json.Unmarshal(matches, &found) != nil {
			return nil
		}
		var nodes []Node
		for _, item := range found {
			nodes = append(nodes, nodeFrom(item.Path, item.Name, item.Type, item.Size, item.Mtime))
		}
		return nodes
	}
	var item struct {
		Path  string `json:"path"`
		Name  string `json:"name"`
		Type  string `json:"type"`
		Size  int64  `json:"size"`
		Mtime string `json:"mtime"`
	}
	if json.Unmarshal([]byte(line), &item) != nil || (item.Type != "file" && item.Type != "dir") {
		return nil
	}
	return []Node{nodeFrom(item.Path, item.Name, item.Type, item.Size, item.Mtime)}
}

func nodeFrom(path, name, kind string, size int64, mtime string) Node {
	win := restore.ToWindows(path)
	if name == "" {
		name = filepathBase(win)
	}
	return Node{Name: name, Path: win, Type: kind, Size: size, Mtime: mtime}
}

func parseRestore(stdout string) RestoreResult {
	var result RestoreResult
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var raw struct {
			MessageType string `json:"message_type"`
			Files       int    `json:"files_restored"`
			Skipped     int    `json:"files_skipped"`
			Error       any    `json:"error"`
			Item        string `json:"item"`
		}
		if json.Unmarshal([]byte(line), &raw) != nil {
			continue
		}
		if raw.MessageType == "summary" {
			result.Files = raw.Files
			result.Skipped = raw.Skipped
		}
		if raw.MessageType == "error" && raw.Item != "" {
			result.Errors = append(result.Errors, raw.Item)
		}
	}
	return result
}

func restoreMessage(code int, stderr string) string {
	if code == 12 {
		return "La contraseña del repositorio no es correcta"
	}
	if code == 10 {
		return "Destino no disponible: conecta el disco USB o comprueba la ruta"
	}
	text := strings.TrimSpace(stderr)
	if text == "" {
		return "No se pudo recuperar la selección"
	}
	if len(text) > 300 {
		text = text[:300]
	}
	return text
}

func filepathDir(path string) string {
	path = strings.TrimRight(path, `\/`)
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '\\' || path[i] == '/' {
			if i == 2 && len(path) > 1 && path[1] == ':' {
				return path[:3]
			}
			return path[:i]
		}
	}
	return path
}

func filepathBase(path string) string {
	path = strings.TrimRight(path, `\/`)
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '\\' || path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
