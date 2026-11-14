// Package diagnose prepara la pantalla de ajustes y el paquete para soporte.
// La contraseña del repositorio no sale ni en el registro ni en el ZIP.
package diagnose

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	maxViewLines = 200
	maxReadBytes = 512 * 1024
	maxZipFile   = 1024 * 1024
	maxReports   = 30
)

// About es lo que se enseña en Ajustes. No incluye secretos.
type About struct {
	AgentVersion  string `json:"agent_version"`
	ResticVersion string `json:"restic_version"`
	DataDir       string `json:"data_dir"`
	PanelPort     int    `json:"panel_port"`
}

// Files son las rutas que entran en el paquete. Se leen, no se modifican.
type Files struct {
	Config     string
	State      string
	ReportsDir string
	LogsDir    string
}

var (
	dpapiRE    = regexp.MustCompile(`(?i)dpapi:[A-Za-z0-9+/=]+`)
	yamlPassRE = regexp.MustCompile(`(?i)(password_encrypted\s*:\s*)(?:"[^"]*"|'[^']*'|[^\s#]+)`)
	jsonPassRE = regexp.MustCompile(`(?i)("(?:password|password_encrypted)"\s*:\s*)"(?:\\.|[^"])*"`)
	envPassRE  = regexp.MustCompile(`(?i)((?:RESTIC_PASSWORD|RESTIC_PASSWORD_FILE|RESTIC_PASSWORD_COMMAND|RESTIC_NEW_PASSWORD|RESTIC_NEW_PASSWORD_FILE)\s*=\s*)\S+`)
	reportName = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}_[0-9]{2}-[0-9]{2}-[0-9]{2}\.[0-9]{3}\.json$`)
	logName    = regexp.MustCompile(`^agent\.log(?:\.[1-5])?$`)
)

// Redact quita contraseñas y blobs DPAPI de un texto.
func Redact(text string) string {
	text = yamlPassRE.ReplaceAllString(text, `${1}""`)
	text = jsonPassRE.ReplaceAllString(text, `${1}""`)
	text = envPassRE.ReplaceAllString(text, `${1}"[omitido]"`)
	text = dpapiRE.ReplaceAllString(text, "[omitido]")
	return text
}

// RedactConfig devuelve el YAML sin la contraseña. Si no se puede interpretar, lo limpia como texto.
func RedactConfig(raw []byte) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err == nil && len(doc.Content) > 0 {
		redactNode(doc.Content[0])
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(&doc); err == nil {
			_ = enc.Close()
			out := Redact(buf.String())
			if strings.Contains(strings.ToLower(out), "dpapi:") {
				return nil, fmt.Errorf("la configuración del paquete todavía tenía un secreto")
			}
			return []byte(out), nil
		}
	}
	out := Redact(string(raw))
	if strings.Contains(strings.ToLower(out), "dpapi:") {
		return nil, fmt.Errorf("la configuración del paquete todavía tenía un secreto")
	}
	return []byte(out), nil
}

func redactNode(n *yaml.Node) {
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range n.Content {
			redactNode(child)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i]
			val := n.Content[i+1]
			name := strings.ToLower(key.Value)
			if name == "password_encrypted" || name == "password" {
				val.Kind = yaml.ScalarNode
				val.Tag = "!!str"
				val.Value = ""
				val.Style = yaml.DoubleQuotedStyle
				val.Content = nil
				continue
			}
			redactNode(val)
		}
	case yaml.ScalarNode:
		if strings.Contains(strings.ToLower(n.Value), "dpapi:") {
			n.Value = Redact(n.Value)
		}
	}
}

// Tail devuelve las últimas líneas del log. level vacío las incluye todas.
func Tail(path, level string, maxLines int) ([]string, error) {
	want, err := normalizeLevel(level)
	if err != nil {
		return nil, err
	}
	if maxLines <= 0 || maxLines > maxViewLines {
		maxLines = maxViewLines
	}
	data, err := readTail(path, maxReadBytes)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("no se pudo leer el registro")
	}
	var matched []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if want != "" && levelOf(line) != want {
			continue
		}
		matched = append(matched, Redact(line))
	}
	if len(matched) > maxLines {
		matched = matched[len(matched)-maxLines:]
	}
	if matched == nil {
		matched = []string{}
	}
	return matched, nil
}

func normalizeLevel(level string) (string, error) {
	level = strings.ToLower(strings.TrimSpace(level))
	switch level {
	case "", "info", "warn", "error":
		return level, nil
	default:
		return "", fmt.Errorf("elige un nivel del registro")
	}
}

func levelOf(line string) string {
	const mark = "level="
	i := strings.Index(line, mark)
	if i < 0 {
		return ""
	}
	rest := line[i+len(mark):]
	end := strings.IndexAny(rest, " \t")
	if end < 0 {
		return strings.ToLower(rest)
	}
	return strings.ToLower(rest[:end])
}

func readTail(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	start := int64(0)
	if info.Size() > max {
		start = info.Size() - max
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(f, max+4096))
	if err != nil {
		return nil, err
	}
	if start > 0 {
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			data = data[i+1:]
		}
	}
	return data, nil
}

// WriteBundle escribe un ZIP con la configuración sin contraseña, informes y logs.
func WriteBundle(w io.Writer, files Files, about About) error {
	zw := zip.NewWriter(w)
	aboutText := fmt.Sprintf("agente: %s\nrestic: %s\ndatos: %s\npuerto: %d\n", about.AgentVersion, about.ResticVersion, about.DataDir, about.PanelPort)
	if err := addText(zw, "acerca.txt", aboutText); err != nil {
		return err
	}
	if files.Config != "" {
		raw, err := os.ReadFile(files.Config)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("no se pudo leer la configuración")
		}
		if err == nil {
			clean, err := RedactConfig(raw)
			if err != nil {
				return err
			}
			if err := addBytes(zw, "config.yaml", clean); err != nil {
				return err
			}
		}
	}
	if files.State != "" {
		if err := addFile(zw, "estado.json", files.State); err != nil {
			return err
		}
	}
	reports, err := newest(files.ReportsDir, reportName, maxReports)
	if err != nil {
		return err
	}
	for _, name := range reports {
		if err := addFile(zw, "informes/"+name, filepath.Join(files.ReportsDir, name)); err != nil {
			return err
		}
	}
	logs, err := newest(files.LogsDir, logName, 6)
	if err != nil {
		return err
	}
	for _, name := range logs {
		if err := addFile(zw, "logs/"+name, filepath.Join(files.LogsDir, name)); err != nil {
			return err
		}
	}
	return zw.Close()
}

func newest(dir string, name *regexp.Regexp, limit int) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("no se pudo leer %s", filepath.Base(dir))
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !name.MatchString(entry.Name()) {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	if len(names) > limit {
		names = names[:limit]
	}
	return names, nil
}

func addFile(zw *zip.Writer, name, path string) error {
	raw, err := readLimited(path, maxZipFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("no se pudo leer %s", filepath.Base(path))
	}
	return addBytes(zw, name, []byte(Redact(string(raw))))
}

func addText(zw *zip.Writer, name, text string) error {
	return addBytes(zw, name, []byte(Redact(text)))
}

func addBytes(zw *zip.Writer, name string, body []byte) error {
	if strings.Contains(strings.ToLower(string(body)), "dpapi:") {
		return fmt.Errorf("el paquete todavía tenía un secreto")
	}
	entry, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = entry.Write(body)
	return err
}

func readLimited(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	start := int64(0)
	if info.Size() > max {
		start = info.Size() - max
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(f, max))
}
