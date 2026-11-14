// Package logging configura el log estructurado del agente, en pantalla y en fichero.
package logging

import (
	"io"
	"log/slog"
	"time"
)

// Options controla dónde se escribe el log.
type Options struct {
	// LogFile es la ruta del fichero activo. Si está vacía, solo se escribe en consola.
	LogFile string
	// Console replica cada evento en la salida estándar.
	Console bool
}

// Logger es el log del agente. Hay que cerrarlo al terminar para soltar el fichero.
type Logger struct {
	*slog.Logger
	file *RotatingFile
}

// New abre el log. Los mensajes van en texto legible, con campos estructurados.
func New(opts Options) (*Logger, error) {
	var writers []io.Writer
	var file *RotatingFile
	if opts.Console {
		writers = append(writers, osStdout)
	}
	if opts.LogFile != "" {
		f, err := NewRotatingFile(opts.LogFile, maxLogBytes, maxLogBackups)
		if err != nil {
			return nil, err
		}
		file = f
		writers = append(writers, f)
	}
	if len(writers) == 0 {
		writers = append(writers, osStdout)
	}

	handler := slog.NewTextHandler(io.MultiWriter(writers...), &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				if tm, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(tm.Local().Format(time.RFC3339))
				}
			}
			return a
		},
	})

	return &Logger{
		Logger: slog.New(handler),
		file:   file,
	}, nil
}

// Close cierra el fichero de log. La salida de consola no se cierra.
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

// osStdout se sustituye en pruebas.
var osStdout io.Writer = stdoutWriter{}
