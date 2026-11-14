//go:build windows

package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProtectTreeHidesTheFolderFromOrdinaryUsers(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "reports")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "config.yaml"), []byte("secreto"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ProtectTree(root); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{root, nested, filepath.Join(nested, "config.yaml")} {
		ok, err := PrivateACL(path)
		if err != nil || !ok {
			t.Fatalf("%s protegida=%v err=%v", path, ok, err)
		}
	}
	if _, err := os.ReadFile(filepath.Join(nested, "config.yaml")); err != nil {
		t.Fatal(err)
	}
}
