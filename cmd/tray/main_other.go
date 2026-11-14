//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "El icono de bandeja solo está disponible en Windows.")
	os.Exit(1)
}
