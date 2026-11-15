package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"backup-agent/internal/cloud"
	"backup-agent/internal/config"
	"backup-agent/internal/paths"
)

// Acciones del panel local sobre la conexión con la plataforma. Solo administrador.

func cloudView() (any, error) {
	sender := initCloud()
	if sender == nil {
		return nil, errors.New("la conexión con la plataforma no está disponible")
	}
	return sender.View(), nil
}

// cloudProblem traduce un fallo del alta o del envío a una frase para la pantalla.
func cloudProblem(err error) string {
	var api *cloud.APIError
	if !errors.As(err, &api) {
		return "No se pudo completar la operación."
	}
	switch api.Code {
	case cloud.CodeInvalidLicense:
		return "La clave de licencia no es válida. Revísala y vuelve a escribirla."
	case cloud.CodeInvalidInstallCode:
		return "El código del instalador no es válido. Escribe la clave de licencia o pide un instalador nuevo."
	case cloud.CodeInstallRevoked, cloud.CodeInstallExpired:
		return "Este instalador ya no vale. Pide uno nuevo al técnico, o escribe la clave de licencia."
	case cloud.CodeLicenseExpired:
		return "La licencia ha caducado. Las copias siguen funcionando con normalidad."
	case cloud.CodeLicenseBlocked:
		return "La licencia está suspendida. Las copias siguen funcionando con normalidad."
	case cloud.CodeLicenseNotStarted:
		return "La licencia todavía no ha empezado."
	case cloud.CodeSeatLimit:
		return "Ya están dados de alta todos los equipos que permite la licencia. Las copias siguen funcionando con normalidad."
	case cloud.CodeMachineRevoked:
		return "Este equipo ha sido desactivado en la plataforma. Las copias siguen funcionando con normalidad."
	case cloud.CodeRateLimited:
		return "Demasiados intentos seguidos. Espera unos minutos y vuelve a probar."
	case cloud.CodeInvalidRequest:
		return "Faltan datos o no son válidos."
	case "bad_server_url":
		return api.Message
	case "machine_uid":
		return "No se pudo identificar este equipo."
	}
	if api.Network {
		return "No se pudo conectar con la plataforma. Revisa la dirección del servidor y la conexión a internet."
	}
	return "La plataforma ha respondido algo inesperado. Comprueba la dirección del servidor."
}

func cloudEnroll(raw []byte) ([]string, error) {
	var body struct {
		Credential string `json:"credential"`
		ServerURL  string `json:"server_url"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return []string{"El formulario no se entiende."}, nil
	}
	sender := initCloud()
	if sender == nil {
		return []string{"La conexión con la plataforma no está disponible."}, nil
	}
	if strings.TrimSpace(body.Credential) == "" {
		return []string{"Escribe la clave de licencia."}, nil
	}
	current := cloudConfig()
	if url := strings.TrimSpace(body.ServerURL); url != "" && url != current.ServerURL {
		current.ServerURL = url
		if err := config.SetCloud(paths.ConfigPath(), current); err != nil {
			var invalid *config.ValidationError
			if errors.As(err, &invalid) {
				return invalid.Problems, nil
			}
			return nil, err
		}
		_ = sender.Forget() // otra plataforma: la clave anterior no vale
	}
	if cloudConfig().ServerURL == "" {
		return []string{"Escribe primero la dirección de la plataforma."}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := sender.Enroll(ctx, body.Credential); err != nil {
		return []string{cloudProblem(err)}, nil
	}
	return nil, nil
}

func cloudSettings(raw []byte) ([]string, error) {
	var body struct {
		ServerURL *string `json:"server_url"`
		SendPaths *bool   `json:"send_paths"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return []string{"El formulario no se entiende."}, nil
	}
	sender := initCloud()
	if sender == nil {
		return []string{"La conexión con la plataforma no está disponible."}, nil
	}
	current := cloudConfig()
	next := current
	if body.ServerURL != nil {
		next.ServerURL = strings.TrimSpace(*body.ServerURL)
	}
	if body.SendPaths != nil {
		next.SendPaths = *body.SendPaths
	}
	if err := config.SetCloud(paths.ConfigPath(), next); err != nil {
		var invalid *config.ValidationError
		if errors.As(err, &invalid) {
			return invalid.Problems, nil
		}
		return nil, err
	}
	if strings.TrimRight(next.ServerURL, "/") != strings.TrimRight(current.ServerURL, "/") {
		_ = sender.Forget() // otra plataforma: hay que dar de alta de nuevo
	}
	signalConfigReload()
	sender.Kick()
	return nil, nil
}

func cloudSync() error {
	sender := initCloud()
	if sender == nil {
		return errors.New("la conexión con la plataforma no está disponible")
	}
	sender.SyncNow()
	return nil
}
