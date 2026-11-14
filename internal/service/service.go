// Package service instala y controla el servicio de Windows del agente.
package service

import (
	"fmt"
	"strings"

	"backup-agent/internal/app"
	winsvc "github.com/kardianos/service"
)

// RunArgument es el argumento con el que el Administrador de servicios arranca el agente.
const RunArgument = "service"

// Config es la definición del servicio. El nombre sale de app.Name.
func Config() *winsvc.Config {
	return &winsvc.Config{
		Name:        app.Name,
		DisplayName: app.Name,
		Description: "Hace las copias de seguridad a las horas configuradas y se detiene al apagar el equipo.",
		Arguments:   []string{RunArgument},
		Option: winsvc.KeyValue{
			"StartType": "automatic",
		},
	}
}

// Control ejecuta install, uninstall, start o stop.
func Control(action string) error {
	s, err := winsvc.New(noop{}, Config())
	if err != nil {
		return err
	}
	return winsvc.Control(s, action)
}

// Run conecta el proceso con el Administrador de servicios.
// En una consola se queda en primer plano hasta Ctrl+C.
func Run(program winsvc.Interface) error {
	s, err := winsvc.New(program, Config())
	if err != nil {
		return err
	}
	return s.Run()
}

// Describe traduce los errores habituales del Administrador de servicios.
func Describe(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "access is denied") || strings.Contains(lower, "acceso denegado"):
		return "hace falta una consola de administrador"
	case strings.Contains(lower, "already exists"):
		return "el servicio ya está instalado"
	case strings.Contains(lower, "is not installed") || strings.Contains(lower, "does not exist"):
		return "el servicio no está instalado"
	case strings.Contains(lower, "has not been started") || strings.Contains(lower, "no se ha iniciado"):
		return "el servicio no está en ejecución"
	case strings.Contains(lower, "already running") || strings.Contains(lower, "already been started"):
		return "el servicio ya está en ejecución"
	default:
		return fmt.Sprintf("no se pudo completar la operación: %s", msg)
	}
}

type noop struct{}

func (noop) Start(winsvc.Service) error { return nil }
func (noop) Stop(winsvc.Service) error  { return nil }
