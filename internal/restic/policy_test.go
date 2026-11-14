package restic

import (
	"strings"
	"testing"
)

func TestCountRemoved(t *testing.T) {
	keep := `[{"host":"PC","remove":null,"keep":[{"id":"aaa"}]}]`
	n, err := countRemoved(keep)
	if err != nil || n != 0 {
		t.Fatalf("keep: %d %v", n, err)
	}
	remove := `[{"host":"PC","keep":[{"id":"aaa"}],"remove":[{"id":"bbb"},{"id":"ccc"}]}]`
	n, err = countRemoved(remove)
	if err != nil || n != 2 {
		t.Fatalf("remove: %d %v", n, err)
	}
}

func TestPlainFailureCoversEveryTechnicalMessage(t *testing.T) {
	lock := `La verificación ha encontrado errores en el repositorio (1): {"message_type":"exit_error","code":11,"message":"unable to create lock in backend: repository is already locked by PID 32256 on ALBERTOM by ALBERTOM\\Alberto\nlock was created at 2026-11-14 14:33:04\nthe unlock command can be used"}`
	got := PlainFailure(lock)
	if strings.Contains(got, "{") || strings.Contains(got, "PID") || strings.Contains(got, "unlock") || !strings.Contains(got, "14:33") {
		t.Fatalf("bloqueo: %s", got)
	}
	password := `{"message_type":"exit_error","code":12,"message":"wrong password or no key found"}`
	if got = PlainFailure(password); got != "La contraseña de las copias no es correcta." {
		t.Fatalf("contraseña: %s", got)
	}
	missing := `Fatal: unable to open config file: The system cannot find the path specified.`
	if got = PlainFailure(missing); !strings.Contains(got, "destino no está disponible") {
		t.Fatalf("destino: %s", got)
	}
	unknown := `{"message_type":"exit_error","code":1,"message":"pack abcd is truncated"}`
	if got = PlainFailure(unknown); got != "No se ha podido completar." || strings.Contains(got, "truncated") {
		t.Fatalf("desconocido: %s", got)
	}
	clear := "Destino no disponible: conecta el disco USB o comprueba la ruta"
	if PlainFailure(clear) != clear {
		t.Fatal("una frase clara no se reescribe")
	}
}

func TestParseCheckSummary(t *testing.T) {
	n, summary := parseCheck("{\"message_type\":\"summary\",\"num_errors\":0,\"suggest_prune\":false}\n")
	if n != 0 || !strings.Contains(summary, "num_errors") {
		t.Fatalf("check: %d %s", n, summary)
	}
	n, _ = parseCheck("{\"message_type\":\"summary\",\"num_errors\":2}")
	if n != 2 {
		t.Fatalf("errores: %d", n)
	}
}

func TestForgetArgsIncludePruneAndNotPassword(t *testing.T) {
	args := forgetArgs(RetentionPolicy{KeepDaily: 7, KeepWeekly: 4, KeepMonthly: 12, Prune: true})
	text := strings.Join(args, " ")
	if !strings.Contains(text, "--prune") || !strings.Contains(text, "--keep-daily 7") {
		t.Fatalf("args: %s", text)
	}
	if strings.Contains(text, "clave") {
		t.Fatal("la contraseña no va en los argumentos")
	}
	without := strings.Join(forgetArgs(RetentionPolicy{KeepDaily: 7, KeepWeekly: 4, KeepMonthly: 12}), " ")
	if strings.Contains(without, "--prune") {
		t.Fatal("sin prune no debe compactar")
	}
}
