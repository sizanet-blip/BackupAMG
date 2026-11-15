package restic

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func replicaExe(t *testing.T) string {
	t.Helper()
	exe := filepath.Join("..", "..", "bin", "restic.exe")
	if _, err := os.Stat(exe); err != nil {
		t.Skip("no está bin/restic.exe")
	}
	return exe
}

func backupOnce(t *testing.T, ctx context.Context, exe, repo, source, password string) {
	t.Helper()
	out, err := Backup(ctx, BackupRequest{Executable: exe, Repository: repo, Password: password, Sources: []string{source}, Host: "PC-PRUEBA"})
	if err != nil || out.Status != StatusSuccess {
		t.Fatalf("copia: %+v %v", out, err)
	}
}

func snapshotCount(t *testing.T, ctx context.Context, exe, repo, password string) int {
	t.Helper()
	list, err := Snapshots(ctx, exe, repo, password)
	if err != nil {
		t.Fatalf("Snapshots: %v", err)
	}
	return len(list)
}

func TestReplicaCopiesSnapshotsAndIsIdempotent(t *testing.T) {
	exe := replicaExe(t)
	const password = "clave-de-prueba"
	root := t.TempDir()
	source := filepath.Join(root, "origen")
	primary := filepath.Join(root, "principal")
	secondary := filepath.Join(root, "secundario")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(text string) {
		if err := os.WriteFile(filepath.Join(source, "a.txt"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if _, err := Init(ctx, InitRequest{Executable: exe, Repository: primary, Password: password}); err != nil {
		t.Fatal(err)
	}
	write("uno")
	backupOnce(t, ctx, exe, primary, source, password)
	write("dos")
	backupOnce(t, ctx, exe, primary, source, password)

	// Sin la copia principal preparada no se puede crear la adicional.
	if _, err := InitReplica(ctx, ReplicaInitRequest{Executable: exe, Repository: secondary, Source: filepath.Join(root, "no-existe"), Password: password}); err == nil {
		t.Fatal("sin principal debía fallar")
	}

	res, err := InitReplica(ctx, ReplicaInitRequest{Executable: exe, Repository: secondary, Source: primary, Password: password})
	if err != nil || res.AlreadyExists || res.ID == "" {
		t.Fatalf("InitReplica: %+v %v", res, err)
	}
	again, err := InitReplica(ctx, ReplicaInitRequest{Executable: exe, Repository: secondary, Source: primary, Password: password})
	if err != nil || !again.AlreadyExists {
		t.Fatalf("segunda InitReplica: %+v %v", again, err)
	}

	copied, err := Copy(ctx, CopyRequest{Executable: exe, Source: primary, Destination: secondary, Password: password})
	if err != nil || copied.Copied != 2 || copied.Cancelled {
		t.Fatalf("primera réplica: %+v %v", copied, err)
	}
	if n := snapshotCount(t, ctx, exe, secondary, password); n != 2 {
		t.Fatalf("la adicional debía tener 2 instantáneas, tiene %d", n)
	}
	// Segunda vez: nada nuevo, nada se repite.
	again2, err := Copy(ctx, CopyRequest{Executable: exe, Source: primary, Destination: secondary, Password: password})
	if err != nil || again2.Copied != 0 {
		t.Fatalf("la réplica repetida no debe copiar nada: %+v %v", again2, err)
	}
	// Una copia nueva se replica sola; las anteriores no se repiten.
	write("tres")
	backupOnce(t, ctx, exe, primary, source, password)
	next, err := Copy(ctx, CopyRequest{Executable: exe, Source: primary, Destination: secondary, Password: password})
	if err != nil || next.Copied != 1 {
		t.Fatalf("réplica incremental: %+v %v", next, err)
	}
	// La copia adicional se abre con la misma contraseña y se puede restaurar desde ella.
	target := filepath.Join(root, "restaurado")
	if _, err := Restore(ctx, exe, secondary, password, "latest", target, nil, true, nil); err != nil {
		t.Fatalf("restaurar desde la adicional: %v", err)
	}
	found := false
	_ = filepath.Walk(target, func(path string, info os.FileInfo, _ error) error {
		if info != nil && !info.IsDir() && info.Name() == "a.txt" {
			data, _ := os.ReadFile(path)
			found = string(data) == "tres"
		}
		return nil
	})
	if !found {
		t.Fatal("no se restauró a.txt con el contenido de la última copia")
	}
}

func TestReplicaCopyErrors(t *testing.T) {
	exe := replicaExe(t)
	root := t.TempDir()
	primary := filepath.Join(root, "principal")
	secondary := filepath.Join(root, "secundario")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := Init(ctx, InitRequest{Executable: exe, Repository: primary, Password: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Copy(ctx, CopyRequest{Executable: exe, Source: primary, Destination: secondary, Password: "a"}); err == nil {
		t.Fatal("copiar a una carpeta sin preparar debía fallar")
	}
	if _, err := Copy(ctx, CopyRequest{Executable: exe, Source: primary, Destination: secondary, Password: " "}); err == nil {
		t.Fatal("sin contraseña debía fallar")
	}
	if _, err := InitReplica(ctx, ReplicaInitRequest{Executable: exe, Repository: secondary, Source: primary, Password: "a"}); err != nil {
		t.Fatal(err)
	}
	// Una contraseña equivocada no es «Copiado»: el error no filtra la contraseña.
	_, err := Copy(ctx, CopyRequest{Executable: exe, Source: primary, Destination: secondary, Password: "equivocada"})
	if err == nil {
		t.Fatal("con otra contraseña debía fallar")
	}
	if strings.Contains(err.Error(), "equivocada") {
		t.Fatalf("el error contiene la contraseña: %v", err)
	}
}

func TestReplicaCopyCancelledIsNotAnError(t *testing.T) {
	exe := replicaExe(t)
	root := t.TempDir()
	primary := filepath.Join(root, "principal")
	secondary := filepath.Join(root, "secundario")
	bg, cancelAll := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelAll()
	if _, err := Init(bg, InitRequest{Executable: exe, Repository: primary, Password: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := InitReplica(bg, ReplicaInitRequest{Executable: exe, Repository: secondary, Source: primary, Password: "a"}); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(bg)
	cancel()
	res, err := Copy(cancelled, CopyRequest{Executable: exe, Source: primary, Destination: secondary, Password: "a"})
	if err != nil || !res.Cancelled {
		t.Fatalf("una réplica cancelada no es un error: %+v %v", res, err)
	}
}

func TestChildEnvDropsInheritedFromSecrets(t *testing.T) {
	base := []string{"PATH=x", "RESTIC_FROM_PASSWORD=heredada", "restic_from_password_file=f", "RESTIC_FROM_REPOSITORY=r", "RESTIC_PASSWORD=vieja"}
	env := childEnv(base, "nueva", "C:/repo")
	joined := strings.Join(env, "\n")
	for _, bad := range []string{"heredada", "restic_from_password_file", "RESTIC_FROM_REPOSITORY", "vieja"} {
		if strings.Contains(joined, bad) {
			t.Fatalf("el entorno conserva %q:\n%s", bad, joined)
		}
	}
	if !strings.Contains(joined, "RESTIC_PASSWORD=nueva") || !strings.Contains(joined, "PATH=x") {
		t.Fatalf("falta lo esencial:\n%s", joined)
	}
}

func TestCountCopied(t *testing.T) {
	out := "snapshot 0c831f94 saved, copied from source snapshot 3391c7b9\nruido\nsnapshot d82c8965 saved, copied from source snapshot 65750b1f\n"
	if n := countCopied(out); n != 2 {
		t.Fatalf("%d", n)
	}
	if countCopied("") != 0 {
		t.Fatal("vacío")
	}
}
