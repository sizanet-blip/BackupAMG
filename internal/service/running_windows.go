//go:build windows

package service

import (
	"backup-agent/internal/app"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// Running indica si el servicio ya está en marcha.
// El estado «arrancando» no cuenta: la configuración puede fallar en ese momento.
func Running() (bool, error) {
	m, err := mgr.Connect()
	if err != nil {
		return false, err
	}
	defer m.Disconnect()

	s, err := m.OpenService(app.Name)
	if err != nil {
		return false, err
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return false, err
	}
	return status.State == svc.Running, nil
}
