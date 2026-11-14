// Package app concentra la identidad del agente.
// El nombre de carpetas, logs y, más adelante, del servicio de Windows sale de aquí.
package app

const (
	// Name es el identificador del agente. Cambiarlo aquí renombra carpetas y servicio.
	Name = "BackupAgent"

	// Version aparece en los logs y, en fases posteriores, en los informes.
	Version = "0.1.0"

	// DataDirEnv, si está definida, sustituye a C:\ProgramData\BackupAgent.
	// Sirve para desarrollo y pruebas sin escribir en ProgramData.
	DataDirEnv = "BACKUP_AGENT_DATA_DIR"
)
