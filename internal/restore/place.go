// Package restore coloca en su sitio los ficheros que restic ha extraído.
package restore

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ToWindows convierte una ruta de restic (/C/Datos/a.txt) en una ruta de Windows.
func ToWindows(path string) string {
	path = strings.TrimSpace(path)
	if len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return filepath.Clean(path)
	}
	slash := strings.ReplaceAll(path, `\`, "/")
	if len(slash) >= 3 && slash[0] == '/' && isLetter(slash[1]) && slash[2] == '/' {
		return filepath.Clean(string(slash[1]) + `:\` + strings.ReplaceAll(slash[3:], "/", `\`))
	}
	extracted := strings.ReplaceAll(path, "/", `\`)
	if len(extracted) >= 1 && isLetter(extracted[0]) && (len(extracted) == 1 || extracted[1] == '\\') && !strings.Contains(extracted, ":") {
		rest := ""
		if len(extracted) > 2 {
			rest = extracted[2:]
		}
		return filepath.Clean(string(extracted[0]) + `:\` + rest)
	}
	return filepath.Clean(extracted)
}

// ToRestic convierte C:\Datos\a.txt en /C/Datos/a.txt.
func ToRestic(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if len(path) >= 2 && path[1] == ':' {
		rest := ""
		if len(path) > 2 {
			rest = strings.TrimPrefix(strings.ReplaceAll(path[2:], `\`, "/"), "/")
		}
		if rest == "" {
			return "/" + string(path[0])
		}
		return "/" + string(path[0]) + "/" + rest
	}
	return strings.ReplaceAll(path, `\`, "/")
}

func isLetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

// Place mueve lo extraído en root a su ruta original.
// Si el destino existe y overwrite es falso, el recuperado se renombra.
func Place(root string, overwrite bool) (files, renamed int, errs []string) {
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			errs = append(errs, err.Error())
			return nil
		}
		dest := ToWindows(rel)
		if len(dest) < 3 || dest[1] != ':' {
			return nil
		}
		if entry.IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				errs = append(errs, dest+": "+err.Error())
			}
			return nil
		}
		final, didRename, err := placeFile(path, dest, overwrite)
		if err != nil {
			errs = append(errs, dest+": "+err.Error())
			return nil
		}
		files++
		if didRename {
			renamed++
			_ = final
		}
		return nil
	})
	if err != nil {
		errs = append(errs, err.Error())
	}
	return files, renamed, errs
}

func placeFile(src, dest string, overwrite bool) (string, bool, error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", false, err
	}
	renamed := false
	target := dest
	if _, err := os.Stat(dest); err == nil {
		if !overwrite {
			target = spareName(dest)
			renamed = true
		} else if err := os.Remove(dest); err != nil {
			return "", false, err
		}
	}
	if err := os.Rename(src, target); err != nil {
		if err := copyFile(src, target); err != nil {
			return "", false, err
		}
		_ = os.Remove(src)
	}
	return target, renamed, nil
}

func spareName(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	candidate := base + " (restaurado)" + ext
	for i := 2; exists(candidate); i++ {
		candidate = fmt.Sprintf("%s (restaurado %d)%s", base, i, ext)
	}
	return candidate
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
