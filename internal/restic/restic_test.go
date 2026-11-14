package restic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBackupFixtures(t *testing.T) {
	success := outcomeFrom(0, mustLines(t, "backup_success.jsonl"), "", false)
	if success.Status != StatusSuccess || success.SnapshotID == "" {
		t.Fatalf("éxito: %+v", success)
	}
	if success.FilesNew != 2 || success.TotalFilesProcessed != 2 || success.DataAdded != 7298 {
		t.Fatalf("resumen: %+v", success)
	}
	if success.Message != "Copia completada correctamente" {
		t.Fatalf("mensaje: %s", success.Message)
	}
	if got := success.Text(); !strings.Contains(got, success.SnapshotID) {
		t.Fatalf("texto: %s", got)
	}

	warning := outcomeFrom(3, mustLines(t, "backup_warning.jsonl"), "", false)
	if warning.Status != StatusWarning {
		t.Fatalf("aviso: %+v", warning)
	}
	if len(warning.Skipped) != 1 || !strings.Contains(warning.Skipped[0].Path, "bloqueado.txt") {
		t.Fatalf("omitidos: %+v", warning.Skipped)
	}
	if !strings.Contains(warning.Skipped[0].Reason, "siendo utilizado por otro proceso") {
		t.Fatalf("motivo: %s", warning.Skipped[0].Reason)
	}
	if !strings.Contains(warning.Message, "con avisos") || warning.SnapshotID == "" {
		t.Fatalf("aviso incompleto: %+v", warning)
	}

	bad := outcomeFrom(12, mustLines(t, "backup_bad_password.jsonl"), "", false)
	if bad.Status != StatusFailed || !strings.Contains(bad.Message, "contraseña") {
		t.Fatalf("contraseña: %+v", bad)
	}

	fatal := outcomeFrom(1, nil, "Fatal: unable to create lock", false)
	if fatal.Status != StatusFailed || !strings.Contains(fatal.Message, "unable to create lock") {
		t.Fatalf("fallo: %+v", fatal)
	}
	if strings.Contains(fatal.Message, "Fatal:") {
		t.Fatalf("no debe repetir el prefijo de restic: %s", fatal.Message)
	}

	empty := outcomeFrom(0, nil, "", false)
	if empty.Status != StatusFailed {
		t.Fatal("un código 0 sin resumen no es un éxito")
	}
}

func TestExitCodes(t *testing.T) {
	cases := map[int]string{
		10: "Destino no disponible",
		11: "bloqueado por otra copia",
		12: "contraseña",
	}
	for code, want := range cases {
		msg := userMessage(code, "Fatal: ignored", false)
		if !strings.Contains(msg, want) {
			t.Fatalf("código %d: %s", code, msg)
		}
	}
}

func TestBackupArgsOmitPasswordAndToggleVSS(t *testing.T) {
	with := backupArgs(BackupRequest{
		Password: "no-debe-aparecer",
		Host:     "PC-1",
		UseVSS:   true,
		Excludes: []string{"*.tmp"},
		Sources:  []string{`C:\Datos`},
	})
	text := strings.Join(with, "\n")
	if strings.Contains(text, "no-debe-aparecer") {
		t.Fatal("la contraseña no puede ir en los argumentos")
	}
	if !strings.Contains(text, "--use-fs-snapshot") || !strings.Contains(text, "--host") || !strings.Contains(text, "*.tmp") {
		t.Fatalf("argumentos: %v", with)
	}
	without := backupArgs(BackupRequest{Sources: []string{`C:\Datos`}})
	if strings.Contains(strings.Join(without, " "), "--use-fs-snapshot") {
		t.Fatal("sin VSS no debe pedirse la instantánea")
	}
}

func TestChildEnvReplacesPassword(t *testing.T) {
	env := childEnv([]string{"RESTIC_PASSWORD=vieja", "PATH=C:\\Windows", "RESTIC_REPOSITORY=Z:\\otro"}, "nueva", `E:\Copias`)
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "vieja") || strings.Contains(joined, `Z:\otro`) {
		t.Fatalf("quedó un valor anterior: %s", joined)
	}
	if strings.Count(joined, "RESTIC_PASSWORD=") != 1 || !strings.Contains(joined, "RESTIC_PASSWORD=nueva") {
		t.Fatalf("entorno: %s", joined)
	}
}

func TestLocationAvailable(t *testing.T) {
	dir := t.TempDir()
	if err := locationAvailable(filepath.Join(dir, "repo-nuevo")); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "fichero")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := locationAvailable(file); err == nil {
		t.Fatal("un fichero no puede ser el destino")
	}
}

func mustLines(t *testing.T, name string) []Line {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var lines []Line
	for _, text := range strings.Split(string(body), "\n") {
		line, ok := parseLine(text)
		if !ok && strings.TrimSpace(text) != "" {
			t.Fatalf("línea no reconocida: %s", text)
		}
		if ok {
			lines = append(lines, line)
		}
	}
	return lines
}
