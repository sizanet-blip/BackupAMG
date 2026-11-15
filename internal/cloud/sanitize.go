package cloud

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"strings"
	"unicode"
)

const (
	maxIssues       = 200
	maxMessage      = 500
	maxReason       = 255
	maxReportBytes  = 256 * 1024
	pathPlaceholder = "[ruta]"
)

// PrepareReport convierte el JSON de un informe local en el que se envía a la plataforma.
// El agente es la barrera principal de la privacidad: por defecto no sale ninguna ruta completa.
//
//   - Se quitan «sources» y «repository» (son rutas del cliente).
//   - skipped_files[].path se envía como «<huella>.<extensión>»; con sendPaths=true, la ruta real.
//   - errors, message y los motivos pierden las rutas que lleven.
//   - Se limita el número de incidencias y el tamaño total (256 KB).
//
// uid es el report_uid si el informe no lo trae (informes anteriores a esta versión).
func PrepareReport(raw []byte, uid string, sendPaths bool) (json.RawMessage, error) {
	var rep map[string]any
	if err := json.Unmarshal(raw, &rep); err != nil {
		return nil, err
	}
	if existing, _ := rep["report_uid"].(string); strings.TrimSpace(existing) == "" {
		rep["report_uid"] = uid
	}
	delete(rep, "sources")
	delete(rep, "repository")

	if list, ok := rep["skipped_files"].([]any); ok {
		out := make([]any, 0, len(list))
		for _, item := range list {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			p, _ := entry["path"].(string)
			reason, _ := entry["reason"].(string)
			sent := p
			if !sendPaths {
				sent = HashedPath(p)
			}
			out = append(out, map[string]any{"path": sent, "reason": cut(scrub(reason, sendPaths), maxReason)})
			if len(out) >= maxIssues {
				break
			}
		}
		rep["skipped_files"] = out
		if len(list) > len(out) {
			rep["skipped_total"] = len(list)
		}
	}
	if list, ok := rep["errors"].([]any); ok {
		out := make([]any, 0, len(list))
		for _, item := range list {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				out = append(out, cut(scrub(text, sendPaths), maxReason))
			}
			if len(out) >= maxIssues {
				break
			}
		}
		rep["errors"] = out
	}
	if msg, ok := rep["message"].(string); ok {
		rep["message"] = cut(scrub(msg, sendPaths), maxMessage)
	}

	out, err := json.Marshal(rep)
	if err != nil {
		return nil, err
	}
	// Informe demasiado grande: se recortan las incidencias hasta que quepa.
	for len(out) > maxReportBytes {
		list, _ := rep["skipped_files"].([]any)
		if len(list) == 0 {
			errs, _ := rep["errors"].([]any)
			if len(errs) == 0 {
				break
			}
			rep["errors"] = errs[:len(errs)/2]
		} else {
			rep["skipped_files"] = list[:len(list)/2]
		}
		if out, err = json.Marshal(rep); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// HashedPath es lo que se envía en lugar de una ruta: la huella de la ruta y su extensión.
func HashedPath(p string) string {
	norm := strings.ToLower(strings.ReplaceAll(p, "\\", "/"))
	sum := sha256.Sum256([]byte(norm))
	name := hex.EncodeToString(sum[:])
	if ext := path.Ext(path.Base(strings.ReplaceAll(p, "\\", "/"))); ext != "" && len(ext) <= 16 && !strings.ContainsAny(ext, " /") {
		name += strings.ToLower(ext)
	}
	return name
}

func cut(text string, max int) string {
	r := []rune(text)
	if len(r) > max {
		return string(r[:max])
	}
	return text
}

// scrub sustituye las rutas de Windows que haya dentro de un texto por «[ruta]».
// Una ruta empieza en «C:\» o «\\servidor» y llega hasta una comilla, «<>|», un salto de línea
// o «: » (dos puntos y espacio, que suelen separar la ruta del motivo). Se prefiere perder algo de
// texto a dejar pasar el nombre de un cliente.
func scrub(text string, keepPaths bool) string {
	if keepPaths || text == "" {
		return text
	}
	runes := []rune(text)
	var b strings.Builder
	for i := 0; i < len(runes); {
		if !startsPath(runes, i) {
			b.WriteRune(runes[i])
			i++
			continue
		}
		j := i
		for j < len(runes) {
			c := runes[j]
			if c == '"' || c == '\'' || c == '<' || c == '>' || c == '|' || c == '\r' || c == '\n' {
				break
			}
			if c == ':' && j > i+1 && (j+1 >= len(runes) || unicode.IsSpace(runes[j+1])) {
				break
			}
			j++
		}
		b.WriteString(pathPlaceholder)
		i = j
	}
	return b.String()
}

func startsPath(r []rune, i int) bool {
	if i+2 < len(r) && r[i+1] == ':' && (r[i+2] == '\\' || r[i+2] == '/') && isLetter(r[i]) {
		return i == 0 || !(isLetter(r[i-1]) || unicode.IsDigit(r[i-1]))
	}
	return i+1 < len(r) && r[i] == '\\' && r[i+1] == '\\'
}

func isLetter(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
