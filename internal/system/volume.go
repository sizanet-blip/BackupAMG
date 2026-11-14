package system

import "strings"

// JoinRepo cambia la letra de una carpeta según el disco donde está montado ahora.
// Una ruta de red se deja igual.
func JoinRepo(stored, mount string) string {
	stored = strings.ReplaceAll(strings.TrimSpace(stored), "/", `\`)
	if strings.HasPrefix(stored, `\\`) && !strings.HasPrefix(strings.ToLower(stored), `\\?\volume{`) {
		return stored
	}
	mount = strings.TrimRight(strings.ReplaceAll(mount, "/", `\`), `\`) + `\`
	rest := strings.Trim(repoSuffix(stored), `\`)
	if rest == "" {
		return mount
	}
	return mount + rest
}

func repoSuffix(stored string) string {
	if len(stored) >= 2 && stored[1] == ':' {
		stored = stored[2:]
	} else if strings.HasPrefix(strings.ToLower(stored), `\\?\volume{`) {
		if i := strings.Index(stored, `}\`); i >= 0 {
			stored = stored[i+2:]
		}
	}
	return strings.TrimLeft(stored, `\`)
}
