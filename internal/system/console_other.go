//go:build !windows

package system

// EnableUTF8Console no hace nada fuera de Windows.
func EnableUTF8Console() {}
