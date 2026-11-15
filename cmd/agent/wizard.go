package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backup-agent/internal/config"
	"backup-agent/internal/paths"
	"backup-agent/internal/restic"
	"backup-agent/internal/secrets"
	"backup-agent/internal/status"
	"backup-agent/internal/system"
)

func setupStatus() (any, error) {
	_, err := config.Load(paths.ConfigPath())
	host, hostErr := os.Hostname()
	if hostErr != nil || strings.TrimSpace(host) == "" {
		host = "este equipo"
	}
	return map[string]any{"needed": err != nil, "hostname": host}, nil
}

func completeSetup(raw []byte) ([]string, error) {
	if _, err := config.Load(paths.ConfigPath()); err == nil {
		return nil, errors.New("ya hay una configuración")
	}
	var body struct {
		Path         string   `json:"path"`
		Sources      []string `json:"sources"`
		Excludes     []string `json:"excludes"`
		Times        []string `json:"times"`
		Weekdays     []int    `json:"weekdays"`
		Password     string   `json:"password"`
		Confirm      string   `json:"confirm"`
		Acknowledged bool     `json:"acknowledged"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return []string{"El formulario no se entiende."}, nil
	}
	var problems []string
	if strings.TrimSpace(body.Password) == "" {
		problems = append(problems, "La contraseña está vacía.")
	}
	if body.Password != body.Confirm {
		problems = append(problems, "Las contraseñas no coinciden.")
	}
	if !body.Acknowledged {
		problems = append(problems, "Marca que has guardado la contraseña en un lugar seguro.")
	}
	if len(body.Weekdays) == 0 {
		problems = append(problems, "Elige al menos un día.")
	}
	if len(problems) > 0 {
		return problems, nil
	}

	repoPath := filepath.Clean(strings.TrimSpace(body.Path))
	volumeID := ""
	if isDrivePath(repoPath) {
		if system.DriveKind(repoPath) == "remote" {
			return []string{"Esta letra es una unidad de red. El servicio no la ve: escribe \\\\servidor\\carpeta."}, nil
		}
		id, err := system.VolumeGUID(repoPath)
		if err != nil {
			return []string{"No se encuentra ese disco. Conéctalo y elige la carpeta otra vez."}, nil
		}
		volumeID = id
	}
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "este equipo"
	}
	encoded, err := secrets.Protect(body.Password)
	if err != nil {
		return nil, err
	}
	cfg := config.Config{
		Agent: config.Agent{Name: strings.TrimSpace(host)},
		Backup: config.Backup{
			Sources:  body.Sources,
			Excludes: cleanList(body.Excludes),
			UseVSS:   true,
		},
		Repository: config.Repository{Path: repoPath, VolumeID: volumeID, PasswordEncrypted: encoded},
		Schedule: config.Schedule{
			Times:              body.Times,
			Weekdays:           body.Weekdays,
			RunMissedOnStartup: true,
			MaxRetries:         2,
			RetryDelayMinutes:  15,
			StaleAfterDays:     2,
		},
		Retention: config.Retention{KeepDaily: 7, KeepWeekly: 4, KeepMonthly: 12, Prune: true},
		Verify:    config.Verify{EveryDays: 7, ReadDataSubset: "5%"},
	}
	if err := config.Draft(cfg); err != nil {
		var invalid *config.ValidationError
		if errors.As(err, &invalid) {
			return invalid.Problems, nil
		}
		return nil, err
	}
	if err := prepareRepository(repoPath, body.Password); err != nil {
		if errors.Is(err, errRepoOtherPassword) {
			return []string{err.Error()}, nil
		}
		return nil, err
	}
	if err := config.Create(paths.ConfigPath(), cfg); err != nil {
		return nil, err
	}
	signalSetup()
	if err := live.request(status.ActivityBackup); err != nil {
		return nil, err
	}
	return nil, nil
}

// errRepoOtherPassword: la carpeta ya tiene copias hechas con otra contraseña.
var errRepoOtherPassword = errors.New("Esta carpeta ya tiene copias hechas con otra contraseña. Elige otra carpeta o escribe la contraseña original.")

func prepareRepository(path, password string) error {
	exe, err := restic.Executable()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if resticRepo(path) {
		ok, exists, err := restic.PasswordOK(ctx, exe, path, password)
		if err != nil {
			return err
		}
		if exists && !ok {
			return errRepoOtherPassword
		}
		return nil
	}
	_, err = restic.Init(ctx, restic.InitRequest{Executable: exe, Repository: path, Password: password})
	return err
}

func cleanList(values []string) []string {
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
