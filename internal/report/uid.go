package report

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// NewUID devuelve un identificador único para un informe (UUID versión 4).
func NewUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("no se pudo generar un identificador: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// LegacyUID calcula un identificador estable para un informe antiguo, que se guardó sin
// report_uid: sale del identificador del equipo y del nombre del fichero, así que enviar el
// mismo informe dos veces da siempre el mismo valor.
func LegacyUID(machineUID, fileName string) string {
	sum := sha256.Sum256([]byte("backupamg-report|" + machineUID + "|" + fileName))
	return "L" + hex.EncodeToString(sum[:])[:31]
}
