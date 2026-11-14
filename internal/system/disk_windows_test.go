//go:build windows

package system

import "testing"

func TestFreeBytes(t *testing.T) {
	n, err := FreeBytes(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if n <= 0 {
		t.Fatalf("espacio libre: %d", n)
	}
	free, total, err := DiskSpace(t.TempDir())
	if err != nil || free <= 0 || total < free {
		t.Fatalf("espacio del disco: libre %d total %d err %v", free, total, err)
	}
}
