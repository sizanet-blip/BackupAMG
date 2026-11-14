//go:build windows

package restic

import (
	"context"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"backup-agent/internal/system"
	"golang.org/x/sys/windows"
)

func TestSection7NamesRestoreAndOpenFiles(t *testing.T) {
	exe := resticBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	root := t.TempDir()
	source := filepath.Join(root, "origen")
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := []byte("Nómina de marzo. Importe 1.234 €")
	short := filepath.Join(source, "informe año 2026.txt")
	if err := os.WriteFile(short, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	longFile := makeLongPath(t, source)
	if err := os.WriteFile(`\\?\`+longFile, payload, 0o644); err != nil {
		t.Fatalf("ruta larga: %v (longitud %d)", err, len(longFile))
	}
	lockedPath := filepath.Join(source, "buzón.pst")
	if err := os.WriteFile(lockedPath, []byte("contenido del pst"), 0o644); err != nil {
		t.Fatal(err)
	}
	locked, err := exclusiveHandle(lockedPath)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(locked)

	denied := filepath.Join(source, "sin permiso.txt")
	if err := os.WriteFile(denied, []byte("secreto"), 0o644); err != nil {
		t.Fatal(err)
	}
	restoreACL := denyRead(t, denied)
	defer restoreACL()

	big := filepath.Join(source, "grande.bin")
	if err := writeRandom(big, 32<<20); err != nil {
		t.Fatal(err)
	}

	initRepo(t, ctx, exe, repo)
	out, err := Backup(ctx, BackupRequest{
		Executable: exe,
		Repository: repo,
		Password:   section7Password,
		Sources:    []string{source},
		Host:       "PC-PRUEBA",
		UseVSS:     false,
	})
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if out.Status != StatusSuccess && out.Status != StatusWarning {
		t.Fatalf("copia: %+v", out)
	}
	if out.Status == StatusWarning && (!skippedContains(out, "buzón.pst") || !skippedContains(out, "sin permiso.txt")) {
		t.Fatalf("omitidos: %+v", out.Skipped)
	}
	if out.SnapshotID == "" {
		t.Fatalf("sin snapshot: %+v", out)
	}

	target := filepath.Join(root, "restaurado")
	restoreLatest(t, ctx, exe, repo, target)
	if got := readRestored(t, target, "informe año 2026.txt"); string(got) != string(payload) {
		t.Fatalf("restaurado corto = %q", got)
	}
	if got := readRestored(t, target, filepath.Base(longFile)); string(got) != string(payload) {
		t.Fatalf("restaurado largo = %q", got)
	}
	if got := readRestored(t, target, "grande.bin"); len(got) != 32<<20 {
		t.Fatalf("restaurado grande = %d bytes", len(got))
	}
	if out.Status == StatusSuccess {
		if got := readRestored(t, target, "buzón.pst"); string(got) != "contenido del pst" {
			t.Fatalf("el fichero abierto no se restauró: %q", got)
		}
	}
}

func TestSection7OpenFileWithVSS(t *testing.T) {
	if !windows.GetCurrentProcessToken().IsElevated() {
		t.Skip("VSS requiere una consola de administrador")
	}
	exe := resticBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	root := t.TempDir()
	source := filepath.Join(root, "origen")
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("documento abierto")
	path := filepath.Join(source, "abierto.docx")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	handle, err := exclusiveHandle(path)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)

	initRepo(t, ctx, exe, repo)
	out, err := Backup(ctx, BackupRequest{
		Executable: exe,
		Repository: repo,
		Password:   section7Password,
		Sources:    []string{source},
		UseVSS:     true,
	})
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if out.Status != StatusSuccess {
		t.Fatalf("VSS no pudo copiar el fichero abierto: %+v", out)
	}
	target := filepath.Join(root, "restaurado")
	restoreLatest(t, ctx, exe, repo, target)
	if got := readRestored(t, target, "abierto.docx"); string(got) != string(body) {
		t.Fatalf("restaurado = %q", got)
	}
}

func TestSection7MissingDestination(t *testing.T) {
	exe := resticBinary(t)
	letter := freeDriveLetter(t)
	_, err := Backup(context.Background(), BackupRequest{
		Executable: exe,
		Repository: letter + `\Copias`,
		Password:   section7Password,
		Sources:    []string{t.TempDir()},
	})
	if err == nil || !strings.Contains(err.Error(), "Destino no disponible") {
		t.Fatalf("error = %v", err)
	}
}

func TestSection7KillLeavesARecoverableRepository(t *testing.T) {
	exe := resticBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	root := t.TempDir()
	source := filepath.Join(root, "origen")
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeRandom(filepath.Join(source, "en-curso.bin"), 200<<20); err != nil {
		t.Fatal(err)
	}
	initRepo(t, ctx, exe, repo)

	cmd := exec.Command(exe, "backup", "--json", source)
	cmd.Env = childEnv(os.Environ(), section7Password, repo)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	killed := false
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if cmd.ProcessState != nil {
			break
		}
		entries, _ := os.ReadDir(filepath.Join(repo, "locks"))
		for _, entry := range entries {
			if !strings.Contains(entry.Name(), "-tmp-") {
				_ = cmd.Process.Kill()
				killed = true
				break
			}
		}
		if killed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = cmd.Wait()
	if !killed {
		t.Fatal("la copia terminó antes de poder cortarla")
	}

	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	removed, err := ReleaseStale(ctx, exe, repo, section7Password, host, time.Now(), system.PIDAlive)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("bloqueos quitados = %d", removed)
	}
	out, err := Backup(ctx, BackupRequest{
		Executable: exe,
		Repository: repo,
		Password:   section7Password,
		Sources:    []string{source},
		UseVSS:     false,
	})
	if err != nil {
		t.Fatalf("la copia siguiente falló: %v", err)
	}
	if out.Status != StatusSuccess || out.SnapshotID == "" {
		t.Fatalf("copia siguiente: %+v", out)
	}
}

const section7Password = "clave-seccion-7"

func resticBinary(t *testing.T) string {
	t.Helper()
	exe := filepath.Join("..", "..", "bin", "restic.exe")
	if _, err := os.Stat(exe); err != nil {
		t.Skip("no está bin/restic.exe")
	}
	return exe
}

func initRepo(t *testing.T, ctx context.Context, exe, repo string) {
	t.Helper()
	result, err := Init(ctx, InitRequest{Executable: exe, Repository: repo, Password: section7Password})
	if err != nil || result.ID == "" {
		t.Fatalf("init: %+v %v", result, err)
	}
}

func makeLongPath(t *testing.T, root string) string {
	t.Helper()
	dir := root
	for len(dir) < 250 {
		dir = filepath.Join(dir, "carpeta año")
	}
	if err := os.MkdirAll(`\\?\`+dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "nómina final.txt")
}

func exclusiveHandle(path string) (windows.Handle, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(ptr, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
}

func denyRead(t *testing.T, path string) func() {
	t.Helper()
	account := os.Getenv("USERDOMAIN") + `\` + os.Getenv("USERNAME")
	cmd := exec.Command("icacls", path, "/inheritance:r", "/deny", account+":(R)")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("icacls: %v\n%s", err, out)
	}
	return func() {
		_ = exec.Command("icacls", path, "/reset").Run()
	}
}

func writeRandom(path string, size int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := make([]byte, 1<<20)
	left := size
	for left > 0 {
		n := len(buf)
		if n > left {
			n = left
		}
		if _, err := rand.Read(buf[:n]); err != nil {
			return err
		}
		if _, err := f.Write(buf[:n]); err != nil {
			return err
		}
		left -= n
	}
	return nil
}

func skippedContains(out BackupOutcome, name string) bool {
	for _, file := range out.Skipped {
		if strings.Contains(file.Path, name) {
			return true
		}
	}
	return false
}

func restoreLatest(t *testing.T, ctx context.Context, exe, repo, target string) {
	t.Helper()
	cmd := exec.CommandContext(ctx, exe, "restore", "latest", "--target", target)
	cmd.Env = childEnv(os.Environ(), section7Password, repo)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("restore: %v\n%s", err, out)
	}
}

func readRestored(t *testing.T, root, name string) []byte {
	t.Helper()
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == name {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil || found == "" {
		t.Fatalf("no está %s en la restauración (%v)", name, err)
	}
	body, err := os.ReadFile(found)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func freeDriveLetter(t *testing.T) string {
	t.Helper()
	for letter := byte('Z'); letter >= 'D'; letter-- {
		name := string(letter) + `:`
		if _, err := os.Stat(name + `\`); err != nil {
			return name
		}
	}
	t.Fatal("no hay una letra de unidad libre")
	return ""
}
