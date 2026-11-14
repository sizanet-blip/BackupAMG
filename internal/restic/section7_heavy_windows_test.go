//go:build windows

package restic

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestSection7Heavy(t *testing.T) {
	if os.Getenv("BACKUP_AGENT_SECTION7") != "1" {
		t.Skip("definir BACKUP_AGENT_SECTION7=1 para cien mil ficheros y el destino lleno")
	}
	exe := resticBinary(t)

	t.Run("cien mil ficheros", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		root := t.TempDir()
		source := filepath.Join(root, "origen")
		repo := filepath.Join(root, "repo")
		if err := os.Mkdir(source, 0o755); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 100_000; i++ {
			path := filepath.Join(source, fmt.Sprintf("f%06d.txt", i))
			if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		initRepo(t, ctx, exe, repo)
		out, err := Backup(ctx, BackupRequest{
			Executable: exe,
			Repository: repo,
			Password:   section7Password,
			Sources:    []string{source},
			UseVSS:     false,
		})
		if err != nil {
			t.Fatal(err)
		}
		if out.Status != StatusSuccess || out.TotalFilesProcessed < 100_000 {
			t.Fatalf("copia: %+v", out)
		}
	})

	t.Run("destino lleno", func(t *testing.T) {
		if !windows.GetCurrentProcessToken().IsElevated() {
			t.Skip("crear un disco pequeño requiere administrador")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		letter, detach := mountTinyVolume(t)
		defer detach()

		root := t.TempDir()
		source := filepath.Join(root, "origen")
		if err := os.Mkdir(source, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := writeRandom(filepath.Join(source, "no-cabe.bin"), 40<<20); err != nil {
			t.Fatal(err)
		}
		repo := letter + `\repo`
		initRepo(t, ctx, exe, repo)
		out, err := Backup(ctx, BackupRequest{
			Executable: exe,
			Repository: repo,
			Password:   section7Password,
			Sources:    []string{source},
			UseVSS:     false,
		})
		if err != nil {
			t.Fatalf("Backup: %v", err)
		}
		if out.Status != StatusFailed || strings.TrimSpace(out.Message) == "" {
			t.Fatalf("un destino lleno tiene que quedar como fallido: %+v", out)
		}
	})
}

func mountTinyVolume(t *testing.T) (string, func()) {
	t.Helper()
	vhd := filepath.Join(t.TempDir(), "lleno.vhdx")
	letter := strings.TrimSuffix(freeDriveLetter(t), ":")
	create := fmt.Sprintf("create vdisk file=\"%s\" maximum=32\r\nselect vdisk file=\"%s\"\r\nattach vdisk\r\ncreate partition primary\r\nformat fs=ntfs quick\r\nassign letter=%s\r\n", vhd, vhd, letter)
	if out, err := diskpart(create); err != nil {
		t.Fatalf("no se pudo crear el disco de prueba: %v\n%s", err, out)
	}
	detach := func() {
		_, _ = diskpart(fmt.Sprintf("select vdisk file=\"%s\"\r\ndetach vdisk\r\n", vhd))
		_ = os.Remove(vhd)
	}
	return letter + ":", detach
}

func diskpart(script string) (string, error) {
	f, err := os.CreateTemp("", "backupagent-diskpart-*.txt")
	if err != nil {
		return "", err
	}
	path := f.Name()
	_, err = f.WriteString(script)
	closeErr := f.Close()
	if err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return "", closeErr
	}
	defer os.Remove(path)
	cmd := exec.Command(`C:\Windows\System32\diskpart.exe`, "/s", path)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
