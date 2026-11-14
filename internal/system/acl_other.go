//go:build !windows

package system

// ProtectTree no hace nada fuera de Windows.
func ProtectTree(string) error { return nil }

// PrivateACL no hace nada fuera de Windows.
func PrivateACL(string) (bool, error) { return false, nil }
