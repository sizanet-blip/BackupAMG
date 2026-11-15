package cloud

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"

	"backup-agent/internal/paths"
)

// MachineUID es el identificador estable de este equipo para la plataforma web.
// Sale de un hash del MachineGuid de Windows: nunca viaja el GUID real ni datos personales, y
// reinstalar el agente en el mismo equipo da el mismo valor (no gasta plaza).
//
// En modo desarrollo (BACKUP_AGENT_DATA_DIR) se puede forzar con BACKUP_AGENT_MACHINE_UID, para
// probar con «otro equipo». El servicio no usa esa variable.
func MachineUID() (string, error) {
	if paths.Overridden() {
		if forced := strings.TrimSpace(os.Getenv("BACKUP_AGENT_MACHINE_UID")); forced != "" {
			return uidFromGUID(forced), nil
		}
	}
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", fmt.Errorf("no se pudo identificar el equipo: %w", err)
	}
	defer key.Close()
	guid, _, err := key.GetStringValue("MachineGuid")
	if err != nil || strings.TrimSpace(guid) == "" {
		return "", fmt.Errorf("no se pudo identificar el equipo")
	}
	return uidFromGUID(guid), nil
}
