package cloud

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// AltaFileName es el fichero que el instalador de una empresa deja junto al agente.
const AltaFileName = "alta-backupamg.json"

// altaVersion es la versión del formato que entiende este agente.
const altaVersion = 1

// Alta es el contenido de alta-backupamg.json. No lleva la clave de licencia, solo un código de
// instalación de la empresa que el técnico puede revocar.
type Alta struct {
	Version     int    `json:"version"`
	ServerURL   string `json:"server_url"`
	InstallCode string `json:"install_code"`
	CompanyName string `json:"company_name"`
	ExpiresAt   string `json:"expires_at"`
}

// Errores del fichero de alta.
var (
	ErrAltaVersion = errors.New("el fichero de alta es de una versión más nueva que este agente")
	ErrAltaInvalid = errors.New("el fichero de alta no es válido")
)

// ReadAlta lee el fichero. Si no existe, devuelve (nil, nil). Ignora los campos desconocidos y
// rechaza una versión mayor que la que entiende.
func ReadAlta(path string) (*Alta, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("no se pudo leer el fichero de alta: %w", err)
	}
	var alta Alta
	if err := json.Unmarshal(raw, &alta); err != nil {
		return nil, ErrAltaInvalid
	}
	if alta.Version > altaVersion {
		return nil, ErrAltaVersion
	}
	alta.ServerURL = strings.TrimSpace(alta.ServerURL)
	alta.InstallCode = strings.TrimSpace(alta.InstallCode)
	if alta.ServerURL == "" || alta.InstallCode == "" {
		return nil, ErrAltaInvalid
	}
	return &alta, nil
}

// IsInstallCode dice si lo escrito parece un código de instalación (BKI-…) y no una clave de licencia.
func IsInstallCode(credential string) bool {
	clean := strings.ToUpper(strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' {
			return -1
		}
		return r
	}, credential))
	return strings.HasPrefix(clean, "BKI")
}
