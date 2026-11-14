package restic

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// RetentionPolicy es la conservación de instantáneas configurada.
type RetentionPolicy struct {
	KeepDaily   int
	KeepWeekly  int
	KeepMonthly int
	KeepYearly  int
	Prune       bool
}

// RetentionResult es lo que borró restic forget.
type RetentionResult struct {
	Applied          bool
	SnapshotsRemoved int
	ExitCode         int
	Message          string
}

// Forget aplica la política. Con Prune, restic también libera el espacio.
func Forget(ctx context.Context, exe, repo, password string, policy RetentionPolicy) (RetentionResult, error) {
	if policy.KeepDaily == 0 && policy.KeepWeekly == 0 && policy.KeepMonthly == 0 && policy.KeepYearly == 0 {
		return RetentionResult{}, fmt.Errorf("la retención no conserva ninguna copia")
	}
	code, stdout, stderr, runErr := run(ctx, exe, password, repo, forgetArgs(policy), nil)
	if runErr != nil && code == 0 {
		return RetentionResult{}, runErr
	}
	removed, parseErr := countRemoved(stdout)
	if code != 0 && code != 3 {
		return RetentionResult{}, policyError("no se pudo aplicar la retención", code, stderr)
	}
	if parseErr != nil {
		return RetentionResult{}, fmt.Errorf("no se pudo leer el resultado de la retención: %w", parseErr)
	}
	result := RetentionResult{
		Applied:          true,
		SnapshotsRemoved: removed,
		ExitCode:         code,
		Message:          "Retención aplicada",
	}
	if removed > 0 && policy.Prune {
		result.Message = "Retención aplicada y espacio liberado"
	}
	if code == 3 {
		result.Message = "No se pudieron borrar algunas instantáneas antiguas"
	}
	return result, nil
}

func forgetArgs(policy RetentionPolicy) []string {
	args := []string{
		"forget", "--json",
		"--keep-daily", strconv.Itoa(policy.KeepDaily),
		"--keep-weekly", strconv.Itoa(policy.KeepWeekly),
		"--keep-monthly", strconv.Itoa(policy.KeepMonthly),
		"--keep-yearly", strconv.Itoa(policy.KeepYearly),
	}
	if policy.Prune {
		args = append(args, "--prune")
	}
	return args
}

func countRemoved(stdout string) (int, error) {
	text := strings.TrimSpace(strings.TrimPrefix(stdout, "\uFEFF"))
	if text == "" {
		return 0, nil
	}
	if n, err := countRemovedJSON(text); err == nil {
		return n, nil
	}
	total := 0
	found := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] != '[' {
			continue
		}
		n, err := countRemovedJSON(line)
		if err != nil {
			return 0, err
		}
		total += n
		found = true
	}
	if !found {
		return 0, fmt.Errorf("la salida no es la lista de instantáneas")
	}
	return total, nil
}

func countRemovedJSON(text string) (int, error) {
	var groups []struct {
		Remove []struct {
			ID string `json:"id"`
		} `json:"remove"`
	}
	if err := json.Unmarshal([]byte(text), &groups); err != nil {
		return 0, err
	}
	n := 0
	for _, group := range groups {
		n += len(group.Remove)
	}
	return n, nil
}

// CheckResult es el resultado de restic check.
type CheckResult struct {
	Status     string
	ExitCode   int
	NumErrors  int
	Message    string
	Raw        string
	RawSummary string
}

// Check lee una parte de los datos del repositorio y dice si está íntegro.
func Check(ctx context.Context, exe, repo, password, subset string) (CheckResult, error) {
	args := []string{"check", "--json"}
	if strings.TrimSpace(subset) != "" {
		args = append(args, "--read-data-subset", subset)
	}
	code, stdout, stderr, runErr := run(ctx, exe, password, repo, args, nil)
	if runErr != nil && code == 0 {
		return CheckResult{}, runErr
	}
	numErrors, summary := parseCheck(stdout)
	result := CheckResult{ExitCode: code, NumErrors: numErrors, RawSummary: summary}
	if code == 0 && numErrors == 0 {
		result.Status = StatusSuccess
		result.Message = "Verificación correcta: el repositorio no tiene errores"
		return result, nil
	}
	result.Status = StatusFailed
	result.Raw = strings.TrimSpace(stderr)
	result.Message = checkFailure(result.Raw, numErrors)
	return result, nil
}

func checkFailure(stderr string, numErrors int) string {
	if plain := failureSituation(stderr, numErrors); plain != "" {
		return plain
	}
	return "No se ha podido completar."
}

func parseCheck(stdout string) (int, string) {
	var summary string
	numErrors := 0
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] != '{' {
			continue
		}
		var raw struct {
			MessageType string `json:"message_type"`
			NumErrors   int    `json:"num_errors"`
		}
		if json.Unmarshal([]byte(line), &raw) != nil {
			continue
		}
		if raw.MessageType == "summary" {
			summary = line
			numErrors = raw.NumErrors
		}
	}
	return numErrors, summary
}

func policyError(action string, code int, stderr string) error {
	switch code {
	case 10:
		return fmt.Errorf("destino no disponible: conecta el disco USB o comprueba la ruta")
	case 11:
		return fmt.Errorf("el repositorio está bloqueado por otra copia")
	case 12:
		return fmt.Errorf("la contraseña del repositorio no es correcta")
	default:
		detail := cleanResticMessage(stderr)
		if detail == "" {
			detail = fmt.Sprintf("código %d", code)
		}
		return fmt.Errorf("%s: %s", action, detail)
	}
}
