package service

import (
	"errors"
	"testing"

	"backup-agent/internal/app"
)

func TestConfigUsesTheAppNameAndStartsAutomatically(t *testing.T) {
	cfg := Config()
	if cfg.Name != app.Name || cfg.DisplayName != app.Name {
		t.Fatalf("nombre = %s / %s", cfg.Name, cfg.DisplayName)
	}
	if len(cfg.Arguments) != 1 || cfg.Arguments[0] != RunArgument {
		t.Fatalf("argumentos = %#v", cfg.Arguments)
	}
	if cfg.Option["StartType"] != "automatic" {
		t.Fatalf("arranque = %#v", cfg.Option["StartType"])
	}
}

func TestDescribeTranslatesServiceManagerErrors(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{errors.New("Access is denied."), "hace falta una consola de administrador"},
		{errors.New("service BackupAgent already exists"), "el servicio ya está instalado"},
		{errors.New("service BackupAgent is not installed"), "el servicio no está instalado"},
		{errors.New("The specified service does not exist as an installed service."), "el servicio no está instalado"},
		{errors.New("The service has not been started."), "el servicio no está en ejecución"},
		{errors.New("Failed to stop BackupAgent: No se ha iniciado el servicio."), "el servicio no está en ejecución"},
		{errors.New("An instance of the service is already running."), "el servicio ya está en ejecución"},
	}
	for _, tc := range cases {
		if got := Describe(tc.err); got != tc.want {
			t.Errorf("Describe(%q) = %q, quiere %q", tc.err, got, tc.want)
		}
	}
}
