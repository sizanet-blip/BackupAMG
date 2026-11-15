package config

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Cloud es la conexión con la plataforma web central. Todo es opcional: sin servidor, el agente
// funciona exactamente igual que antes. La dirección es un ajuste, nunca está en el código.
type Cloud struct {
	// ServerURL es la dirección base de la plataforma, sin «/backup/v1».
	ServerURL string
	// SendPaths permite enviar las rutas completas de los ficheros omitidos. Por defecto no.
	SendPaths bool
}

type rawCloud struct {
	ServerURL *string `yaml:"server_url"`
	SendPaths *bool   `yaml:"send_paths"`
}

type fileCloud struct {
	ServerURL string `yaml:"server_url,omitempty"`
	SendPaths bool   `yaml:"send_paths,omitempty"`
}

func applyCloud(raw *rawCloud, cfg *Config) []string {
	if raw == nil {
		return nil
	}
	var problems []string
	if raw.ServerURL != nil {
		value := strings.TrimSpace(*raw.ServerURL)
		if value != "" {
			if err := CheckServerURL(value); err != nil {
				problems = append(problems, "cloud.server_url: "+err.Error())
			} else {
				cfg.Cloud.ServerURL = strings.TrimRight(value, "/")
			}
		}
	}
	if raw.SendPaths != nil {
		cfg.Cloud.SendPaths = *raw.SendPaths
	}
	return problems
}

// CheckServerURL comprueba que la dirección del servidor se puede usar: https (http solo a la
// propia máquina, para pruebas), sin usuario ni parámetros.
func CheckServerURL(value string) error {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("usa una dirección completa que empiece por https://, por ejemplo https://panel.ejemplo.com")
	}
	host := u.Hostname()
	local := host == "localhost" || host == "127.0.0.1" || host == "::1"
	if u.Scheme == "https" || (u.Scheme == "http" && local) {
		return nil
	}
	return fmt.Errorf("la dirección tiene que empezar por https://")
}

// SetCloud cambia la sección «cloud» y conserva el resto del fichero. Si no es válido, no se toca.
func SetCloud(path string, c Cloud) error {
	c.ServerURL = strings.TrimSpace(c.ServerURL)
	if c.ServerURL != "" {
		if err := CheckServerURL(c.ServerURL); err != nil {
			return &ValidationError{Problems: []string{"La dirección del servidor no es válida: " + err.Error() + "."}}
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no se encontró la configuración en %s", path)
		}
		return fmt.Errorf("no se pudo leer la configuración %s: %w", path, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("el fichero de configuración no es un YAML válido: %w", err)
	}
	root := mappingRoot(&doc)
	if root == nil {
		return fmt.Errorf("la configuración no es un documento YAML de claves y valores")
	}
	deleteKey(root, "cloud")
	if c.ServerURL != "" || c.SendPaths {
		var node yaml.Node
		if err := node.Encode(fileCloud{ServerURL: strings.TrimRight(c.ServerURL, "/"), SendPaths: c.SendPaths}); err != nil {
			return fmt.Errorf("no se pudo preparar la configuración: %w", err)
		}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "cloud"}, &node)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return fmt.Errorf("no se pudo preparar la configuración: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("no se pudo preparar la configuración: %w", err)
	}
	if _, err := Parse(buf.Bytes()); err != nil {
		return err
	}
	return atomicWrite(path, buf.Bytes())
}

func cloudForFile(c Cloud) *fileCloud {
	if c.ServerURL == "" && !c.SendPaths {
		return nil
	}
	return &fileCloud{ServerURL: c.ServerURL, SendPaths: c.SendPaths}
}
