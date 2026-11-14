package logging

import "os"

// stdoutWriter evita un ciclo de inicialización y deja claro que la consola es os.Stdout.
type stdoutWriter struct{}

func (stdoutWriter) Write(p []byte) (int, error) {
	return os.Stdout.Write(p)
}
