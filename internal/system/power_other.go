//go:build !windows

package system

// PreventSleep no hace nada fuera de Windows.
func PreventSleep() func() { return func() {} }

// PIDAlive no hace nada fuera de Windows.
func PIDAlive(pid int) bool { return false }
