package main

// menuAction identifica cada entrada del menú de la bandeja.
type menuAction int

const (
	menuPanel menuAction = iota
	menuBackup
	menuHistory
	menuRestore
	menuHide
)

// menuEntry es una fila del menú. hint es una línea pequeña de aclaración debajo del texto.
type menuEntry struct {
	action  menuAction
	label   string
	hint    string
	bold    bool
	enabled bool
}

// menuEntries devuelve las filas del menú, en el orden en que se ven. Las cuatro primeras
// necesitan que el servicio esté en marcha; «Ocultar el icono» no, porque solo cierra el icono.
func menuEntries(online, busy bool) []menuEntry {
	return []menuEntry{
		{action: menuPanel, label: "Abrir panel", bold: true, enabled: online},
		{action: menuBackup, label: "Copiar ahora", enabled: online && !busy},
		{action: menuHistory, label: "Ver historial", enabled: online},
		{action: menuRestore, label: "Restaurar archivos", enabled: online},
		{action: menuHide, label: "Ocultar el icono", hint: "Las copias siguen funcionando", enabled: true},
	}
}
