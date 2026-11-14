//go:build windows

package system

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// Shortcut es una carpeta habitual de un usuario.
type Shortcut struct {
	ID   string `json:"id"`
	User string `json:"user"`
	Path string `json:"path"`
}

// Dir es una carpeta que se puede abrir en el explorador.
type Dir struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

var knownFolders = []struct {
	id    string
	names []string
}{
	{"documents", []string{"Documents", "Documentos"}},
	{"desktop", []string{"Desktop", "Escritorio"}},
	{"pictures", []string{"Pictures", "Imágenes", "Imagenes"}},
	{"downloads", []string{"Downloads", "Descargas"}},
	{"videos", []string{"Videos", "Vídeos", "Videos"}},
	{"music", []string{"Music", "Música", "Musica"}},
}

// Shortcuts lista Documentos, Escritorio y el resto de carpetas habituales de cada usuario.
func Shortcuts() []Shortcut {
	root := `C:\Users`
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []Shortcut
	skip := map[string]struct{}{"Public": {}, "Default": {}, "Default User": {}, "All Users": {}}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, omit := skip[entry.Name()]; omit {
			continue
		}
		home := filepath.Join(root, entry.Name())
		seen := map[string]struct{}{}
		for _, folder := range knownFolders {
			for _, name := range folder.names {
				path := filepath.Join(home, name)
				info, err := os.Stat(path)
				if err != nil || !info.IsDir() {
					continue
				}
				key := strings.ToLower(path)
				if _, dup := seen[key]; dup {
					continue
				}
				seen[key] = struct{}{}
				out = append(out, Shortcut{ID: folder.id, User: entry.Name(), Path: path})
				break
			}
		}
	}
	return out
}

// ListDirs devuelve las carpetas hijas. Con path vacío devuelve las unidades.
func ListDirs(path string) ([]Dir, error) {
	if strings.TrimSpace(path) == "" {
		return drives(), nil
	}
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return nil, errNotFolder
	}
	info, err := os.Stat(clean)
	if err != nil || !info.IsDir() {
		return nil, errNotFolder
	}
	entries, err := os.ReadDir(clean)
	if err != nil {
		return nil, errNotFolder
	}
	var out []Dir
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		out = append(out, Dir{Name: entry.Name(), Path: filepath.Join(clean, entry.Name())})
	}
	return out, nil
}

// EstimateBytes suma el tamaño de los ficheros. complete es falso si se agota el tiempo.
func EstimateBytes(ctx context.Context, roots []string) (int64, bool) {
	var total int64
	complete := true
	for _, root := range roots {
		if ctx.Err() != nil {
			return total, false
		}
		clean := filepath.Clean(root)
		if !filepath.IsAbs(clean) {
			continue
		}
		err := filepath.WalkDir(clean, func(path string, entry fs.DirEntry, walkErr error) error {
			if ctx.Err() != nil {
				complete = false
				return context.Canceled
			}
			if walkErr != nil || entry == nil || !entry.Type().IsRegular() {
				return nil
			}
			info, err := entry.Info()
			if err == nil {
				total += info.Size()
			}
			return nil
		})
		if err != nil && ctx.Err() != nil {
			complete = false
		}
	}
	return total, complete
}

func drives() []Dir {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	var out []Dir
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		letter := string(rune('A' + i))
		path := letter + `:\`
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			continue
		}
		out = append(out, Dir{Name: path, Path: path})
	}
	return out
}

var errNotFolder = errFolder("esa ruta no es una carpeta de este equipo")

type errFolder string

func (e errFolder) Error() string { return string(e) }
