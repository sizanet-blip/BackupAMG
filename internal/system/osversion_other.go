//go:build !windows

package system

import "runtime"

// OSVersion es el sistema operativo.
func OSVersion() string { return runtime.GOOS }
