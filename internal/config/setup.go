package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// PeekPort lee el puerto del panel aunque el resto del fichero todavía no sea válido.
func PeekPort(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultPanelPort
	}
	var doc struct {
		Panel *struct {
			Port *int `yaml:"port"`
		} `yaml:"panel"`
	}
	if yaml.Unmarshal(data, &doc) != nil || doc.Panel == nil || doc.Panel.Port == nil {
		return DefaultPanelPort
	}
	port := *doc.Panel.Port
	if port < 1 || port > 65535 {
		return DefaultPanelPort
	}
	return port
}

// Create escribe una configuración nueva. Si ya hay una válida, no la sustituye.
// La contraseña tiene que llegar ya cifrada.
func Create(path string, cfg Config) error {
	if _, err := Load(path); err == nil {
		return errAlreadyConfigured
	}
	if cfg.Panel.Port == 0 {
		cfg.Panel.Port = PeekPort(path)
	}
	raw, err := prepare(cfg)
	if err != nil {
		return err
	}
	return atomicWrite(path, raw)
}

// Draft comprueba un borrador sin escribirlo.
func Draft(cfg Config) error {
	_, err := prepare(cfg)
	return err
}

func prepare(cfg Config) ([]byte, error) {
	cfg.Schedule.Weekdays = normalizeWeekdays(cfg.Schedule.Weekdays)
	if cfg.Panel.Port == 0 {
		cfg.Panel.Port = DefaultPanelPort
	}
	raw, err := marshalFile(cfg)
	if err != nil {
		return nil, err
	}
	if _, err := Parse(raw); err != nil {
		return nil, err
	}
	return raw, nil
}
