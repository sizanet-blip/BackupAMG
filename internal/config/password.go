package config

import (
	"bytes"
	"fmt"
	"os"

	"backup-agent/internal/secrets"
	"gopkg.in/yaml.v3"
)

// SavePassword pide la contraseña dos veces, la cifra y la escribe en el YAML.
// prompt recibe el texto que hay que mostrar y devuelve lo tecleado, sin salto de línea.
func SavePassword(path string, prompt func(label string) (string, error)) error {
	first, err := prompt("Contraseña del repositorio: ")
	if err != nil {
		return err
	}
	second, err := prompt("Repite la contraseña: ")
	if err != nil {
		return err
	}
	if first != second {
		return fmt.Errorf("las contraseñas no coinciden")
	}
	if first == "" {
		return fmt.Errorf("la contraseña está vacía")
	}
	encoded, err := secrets.Protect(first)
	if err != nil {
		return err
	}
	return SetPasswordEncrypted(path, encoded)
}

// SetPasswordEncrypted sustituye repository.password_encrypted sin reescribir el resto a mano.
func SetPasswordEncrypted(path, encoded string) error {
	if !secrets.Valid(encoded) {
		return fmt.Errorf("el valor de la contraseña no tiene el formato cifrado esperado")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no se encontró la configuración en %s. Copia configs\\config.example.yaml a esa ruta y ajústala", path)
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
	setScalar(repo, "password_encrypted", encoded)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return fmt.Errorf("no se pudo preparar la configuración: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("no se pudo preparar la configuración: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("no se pudo escribir la configuración: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("no se pudo reemplazar la configuración: %w", err2)
		}
	}
	return nil
}

func mappingRoot(doc *yaml.Node) *yaml.Node {
	if doc == nil {
		return nil
	}
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) != 1 {
			return nil
		}
		doc = doc.Content[0]
	}
	if doc.Kind != yaml.MappingNode {
		return nil
	}
	return doc
}

func ensureMapping(parent *yaml.Node, key string) *yaml.Node {
	if value := mappingValue(parent, key); value != nil && value.Kind == yaml.MappingNode {
		return value
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valueNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if existing := mappingValue(parent, key); existing != nil {
		replaceMappingValue(parent, key, valueNode)
		return valueNode
	}
	parent.Content = append(parent.Content, keyNode, valueNode)
	return valueNode
}

func setScalar(parent *yaml.Node, key, value string) {
	scalar := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value, Style: yaml.DoubleQuotedStyle}
	if current := mappingValue(parent, key); current != nil {
		current.Kind = yaml.ScalarNode
		current.Tag = "!!str"
		current.Value = value
		current.Style = yaml.DoubleQuotedStyle
		current.Content = nil
		return
	}
	parent.Content = append(parent.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		scalar,
	)
}

func mappingValue(parent *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			return parent.Content[i+1]
		}
	}
	return nil
}

func replaceMappingValue(parent *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			parent.Content[i+1] = value
			return
		}
	}
}
