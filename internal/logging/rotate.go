package logging

import (
	"fmt"
	"os"
	"sync"
)

// maxLogBytes y maxLogBackups limitan el crecimiento del log del agente.
const (
	maxLogBytes   int64 = 10 * 1024 * 1024
	maxLogBackups       = 5
)

// RotatingFile escribe en un fichero y, al superar maxBytes, conserva
// hasta maxBackups copias con sufijo .1, .2, …
type RotatingFile struct {
	mu         sync.Mutex
	path       string
	maxBytes   int64
	maxBackups int
	file       *os.File
	size       int64
}

// NewRotatingFile abre (o crea) el fichero de log activo.
func NewRotatingFile(path string, maxBytes int64, maxBackups int) (*RotatingFile, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("el tamaño máximo del log debe ser mayor que cero")
	}
	if maxBackups < 1 {
		return nil, fmt.Errorf("debe conservarse al menos una copia del log")
	}
	r := &RotatingFile{
		path:       path,
		maxBytes:   maxBytes,
		maxBackups: maxBackups,
	}
	if err := r.open(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *RotatingFile) open() error {
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("no se pudo abrir el fichero de log %s: %w", r.path, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("no se pudo leer el tamaño de %s: %w", r.path, err)
	}
	r.file = f
	r.size = info.Size()
	return nil
}

// Write implementa io.Writer. Rota antes de escribir si la línea no cabe.
func (r *RotatingFile) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.file == nil {
		if err := r.open(); err != nil {
			return 0, err
		}
	}
	if r.size > 0 && r.size+int64(len(p)) > r.maxBytes {
		if err := r.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := r.file.Write(p)
	r.size += int64(n)
	if err != nil {
		return n, fmt.Errorf("no se pudo escribir en el log: %w", err)
	}
	return n, nil
}

// Close cierra el fichero activo.
func (r *RotatingFile) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	if err != nil {
		return fmt.Errorf("no se pudo cerrar el log: %w", err)
	}
	return nil
}

func (r *RotatingFile) rotate() error {
	if err := r.file.Close(); err != nil {
		r.file = nil
		return fmt.Errorf("no se pudo cerrar el log antes de rotarlo: %w", err)
	}
	r.file = nil

	oldest := backupPath(r.path, r.maxBackups)
	if err := os.Remove(oldest); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("no se pudo eliminar %s: %w", oldest, err)
	}
	for i := r.maxBackups - 1; i >= 1; i-- {
		src := backupPath(r.path, i)
		dst := backupPath(r.path, i+1)
		if _, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("no se pudo comprobar %s: %w", src, err)
		}
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("no se pudo archivar %s: %w", src, err)
		}
	}
	if err := os.Rename(r.path, backupPath(r.path, 1)); err != nil {
		return fmt.Errorf("no se pudo archivar el log activo: %w", err)
	}
	return r.open()
}

func backupPath(path string, n int) string {
	return fmt.Sprintf("%s.%d", path, n)
}
