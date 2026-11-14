package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SetDestination cambia la carpeta de destino y, si es un disco, su identificador.
// La contraseña y el resto de la configuración se conservan. Si no es válido, el fichero no se toca.
func SetDestination(path, repoPath, volumeID string) error {
	repoPath = cleanDestination(repoPath)
	volumeID = strings.TrimSpace(volumeID)
	var problems []string
	if repoPath == "" || !absoluteWindowsPath(repoPath) {
		problems = append(problems, `repository.path: la ruta debe ser absoluta o UNC, por ejemplo E:\Copias o \\nas\copias`)
	}
	if volumeID != "" && !validVolumeID(volumeID) {
		problems = append(problems, "repository.volume_id: el identificador del disco no es válido")
	}
	if volumeID != "" && uncPath(repoPath) {
		problems = append(problems, "repository.volume_id: una carpeta de red no usa identificador de disco")
	}
	if len(problems) > 0 {
		return &ValidationError{Problems: problems}
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
	repo := ensureMapping(root, "repository")
	setScalar(repo, "path", repoPath)
	if volumeID == "" {
		deleteKey(repo, "volume_id")
	} else {
		setScalar(repo, "volume_id", volumeID)
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

func cleanDestination(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

func deleteKey(parent *yaml.Node, key string) {
	next := make([]*yaml.Node, 0, len(parent.Content))
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			continue
		}
		next = append(next, parent.Content[i], parent.Content[i+1])
	}
	parent.Content = next
}
