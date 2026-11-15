// Package cloud conecta el agente con la plataforma web central de BackupAMG.
//
// Regla principal: la plataforma solo se informa; nunca puede hacer que el agente deje de copiar.
// Un fallo de red, una licencia caducada, un límite de equipos o un equipo revocado solo cambian el
// estado que se muestra y detienen el envío. Todo se hace en un hilo aparte.
package cloud

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// uidFromGUID es el valor que se envía como machine_uid: sha256("backupamg|v1|" + guid), en hexadecimal.
func uidFromGUID(guid string) string {
	sum := sha256.Sum256([]byte("backupamg|v1|" + strings.TrimSpace(guid)))
	return hex.EncodeToString(sum[:])
}
