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
