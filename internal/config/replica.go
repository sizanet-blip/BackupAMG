package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Tipos de copia adicional. La nube (s3) todavía no existe: se rechaza con un mensaje claro.
const (
	ReplicaLocal = "local"
	ReplicaUNC   = "unc"

	// ReplicaAfterBackup replica tras cada copia principal correcta.
	ReplicaAfterBackup = "after_backup"
	// ReplicaDailyAt replica una vez al día a la hora indicada.
	ReplicaDailyAt = "daily_at"
)

var replicaIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)

// Replica es una copia adicional: un repositorio secundario donde se replica el principal.
type Replica struct {
	ID       string
	Name     string
	Type     string
	Path     string
	VolumeID string
	Enabled  bool
	When     string
	// Time es la hora HH:MM de la réplica diaria. Vacío con after_backup.
	Time string
}

type rawReplica struct {
	ID       *string `yaml:"id"`
	Name     *string `yaml:"name"`
	Type     *string `yaml:"type"`
	Path     *string `yaml:"path"`
	VolumeID *string `yaml:"volume_id"`
	Enabled  *bool   `yaml:"enabled"`
	When     *string `yaml:"when"`
	Time     *string `yaml:"time"`
}

// fileReplica es la forma en que se escribe una copia adicional en el YAML.
type fileReplica struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name,omitempty"`
	Type     string `yaml:"type"`
	Path     string `yaml:"path"`
	VolumeID string `yaml:"volume_id,omitempty"`
	Enabled  bool   `yaml:"enabled"`
	When     string `yaml:"when"`
	Time     string `yaml:"time,omitempty"`
}

func fileReplicas(list []Replica) []fileReplica {
	if len(list) == 0 {
		return nil
	}
	out := make([]fileReplica, 0, len(list))
	for _, r := range list {
		out = append(out, fileReplica{ID: r.ID, Name: r.Name, Type: r.Type, Path: r.Path, VolumeID: r.VolumeID, Enabled: r.Enabled, When: r.When, Time: r.Time})
	}
	return out
}

// applyReplicas valida las copias adicionales. Se llama con el resto de la configuración ya leída,
// porque compara con el destino principal y con los orígenes.
func applyReplicas(raw []rawReplica, cfg *Config) []string {
	var problems []string
	ids := map[string]struct{}{}
	for i, item := range raw {
		field := fmt.Sprintf("replicas[%d]", i)
		var r Replica
		if item.ID != nil {
			r.ID = strings.TrimSpace(*item.ID)
		}
		if !replicaIDPattern.MatchString(r.ID) {
			problems = append(problems, field+".id: usa solo letras minúsculas, números, guion y guion bajo (hasta 40 caracteres)")
		} else if _, dup := ids[r.ID]; dup {
			problems = append(problems, field+".id: el identificador "+r.ID+" está repetido")
		} else {
			ids[r.ID] = struct{}{}
		}
		if item.Name != nil {
			r.Name = strings.TrimSpace(*item.Name)
		}
		if strings.ContainsAny(r.Name, "\r\n") {
			problems = append(problems, field+".name: no puede ocupar más de una línea")
		}
		if r.Name == "" {
			r.Name = r.ID
		}
		if item.Path != nil {
			r.Path = strings.TrimSpace(*item.Path)
		}
		pathOK := false
		switch {
		case r.Path == "":
			problems = append(problems, field+`.path: indica la carpeta de la copia adicional, por ejemplo F:\CopiasExtra o \\nas\copias`)
		case !absoluteWindowsPath(r.Path):
			problems = append(problems, field+`.path: la ruta debe ser absoluta o UNC, por ejemplo F:\CopiasExtra o \\nas\copias`)
		default:
			r.Path = filepath.Clean(r.Path)
			pathOK = true
		}
		kind := ""
		if item.Type != nil {
			kind = strings.ToLower(strings.TrimSpace(*item.Type))
		}
		switch kind {
		case "":
			if pathOK && uncPath(r.Path) {
				r.Type = ReplicaUNC
			} else {
				r.Type = ReplicaLocal
			}
		case ReplicaLocal, ReplicaUNC:
			r.Type = kind
			if pathOK && (kind == ReplicaUNC) != uncPath(r.Path) {
				problems = append(problems, field+".type: no coincide con la ruta (local para un disco, unc para una carpeta de red)")
			}
		case "s3":
			problems = append(problems, field+".type: las copias en la nube todavía no están disponibles")
		default:
			problems = append(problems, fmt.Sprintf("%s.type: tipo no válido %q (usa local o unc)", field, kind))
		}
		if item.VolumeID != nil {
			id := strings.TrimSpace(*item.VolumeID)
			switch {
			case id == "":
			case !validVolumeID(id):
				problems = append(problems, field+".volume_id: el identificador del disco no es válido")
			case pathOK && uncPath(r.Path):
				problems = append(problems, field+".volume_id: una carpeta de red no usa identificador de disco")
			default:
				r.VolumeID = id
			}
		}
		r.Enabled = true
		if item.Enabled != nil {
			r.Enabled = *item.Enabled
		}
		r.When = ReplicaAfterBackup
		if item.When != nil && strings.TrimSpace(*item.When) != "" {
			r.When = strings.ToLower(strings.TrimSpace(*item.When))
		}
		switch r.When {
		case ReplicaAfterBackup:
		case ReplicaDailyAt:
			value := ""
			if item.Time != nil {
				value = *item.Time
			}
			normalized, ok := normalizeTime(value)
			if !ok {
				problems = append(problems, fmt.Sprintf("%s.time: la réplica diaria necesita una hora HH:MM válida (recibido %q)", field, strings.TrimSpace(value)))
			}
			r.Time = normalized
		default:
			problems = append(problems, fmt.Sprintf("%s.when: valor no válido %q (usa after_backup o daily_at)", field, r.When))
		}
		if pathOK {
			problems = append(problems, replicaPlacement(field, r, cfg)...)
			for _, other := range cfg.Replicas {
				if PathsOverlap(r.Path, other.Path) {
					problems = append(problems, fmt.Sprintf("%s.path: se solapa con la copia adicional «%s»", field, other.Name))
				}
			}
		}
		cfg.Replicas = append(cfg.Replicas, r)
	}
	return problems
}

// replicaPlacement comprueba que la copia adicional no está en el principal ni en un origen.
func replicaPlacement(field string, r Replica, cfg *Config) []string {
	var problems []string
	if cfg.Repository.Path != "" && PathsOverlap(r.Path, cfg.Repository.Path) {
		problems = append(problems, field+".path: la copia adicional no puede ser la misma carpeta que la copia principal, ni estar dentro de ella ni contenerla")
	}
	for _, source := range cfg.Backup.Sources {
		if PathsOverlap(r.Path, source) {
			problems = append(problems, field+".path: la copia adicional no puede estar dentro de una carpeta que se copia ("+source+") ni contenerla")
			break
		}
	}
	return problems
}

// PathsOverlap dice si dos rutas son la misma o una está dentro de la otra (sin distinguir mayúsculas).
func PathsOverlap(a, b string) bool {
	a = strings.ToLower(filepath.Clean(strings.ReplaceAll(a, "/", `\`)))
	b = strings.ToLower(filepath.Clean(strings.ReplaceAll(b, "/", `\`)))
	a, b = strings.TrimRight(a, `\`), strings.TrimRight(b, `\`)
	if a == b {
		return true
	}
	sep := `\`
	return strings.HasPrefix(a+sep, b+sep) || strings.HasPrefix(b+sep, a+sep)
}

// ValidateReplicas aplica las mismas reglas del fichero a una lista nueva sin escribir nada.
// Sirve para que el panel rechace una copia adicional antes de guardarla.
func ValidateReplicas(current *Config, list []Replica) error {
	probe := *current
	probe.Replicas = nil
	raw := make([]rawReplica, 0, len(list))
	for i := range list {
		r := list[i]
		raw = append(raw, rawReplica{ID: &r.ID, Name: &r.Name, Type: &r.Type, Path: &r.Path, VolumeID: &r.VolumeID, Enabled: &r.Enabled, When: &r.When, Time: &r.Time})
	}
	if problems := applyReplicas(raw, &probe); len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}
	return nil
}

// SetReplicas sustituye la lista de copias adicionales y conserva el resto del fichero tal cual
// (comentarios, contraseña, orden). Con la lista vacía quita la sección. Si el resultado no es
// válido, el fichero no se toca.
func SetReplicas(path string, list []Replica) error {
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
	deleteKey(root, "replicas")
	if len(list) > 0 {
		var node yaml.Node
		if err := node.Encode(fileReplicas(list)); err != nil {
			return fmt.Errorf("no se pudo preparar la configuración: %w", err)
		}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "replicas"},
			&node,
		)
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
