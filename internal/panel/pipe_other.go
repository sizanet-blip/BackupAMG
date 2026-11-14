//go:build !windows

package panel

import (
	"context"
	"fmt"

	"backup-agent/internal/status"
)

func (s *Server) servePipe(context.Context) {}

// RequestToken no está disponible fuera de Windows.
func RequestToken() (string, string, Role, error) {
	return "", "", "", fmt.Errorf("el panel solo está disponible en Windows")
}

// RequestStatus no está disponible fuera de Windows.
func RequestStatus() (status.View, error) {
	return status.View{}, fmt.Errorf("el panel solo está disponible en Windows")
}

// RequestBackup no está disponible fuera de Windows.
func RequestBackup() error {
	return fmt.Errorf("el panel solo está disponible en Windows")
}
