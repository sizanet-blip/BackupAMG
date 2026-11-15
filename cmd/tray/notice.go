package main

import (
	"strconv"
	"strings"
	"time"

	"backup-agent/internal/status"
)

type noticeOptions struct {
	warnings bool
	success  bool
}

// avisoAccion cierra cada notificación: pulsarla abre el panel. Un aviso del icono de Windows
// no admite botones, así que la acción es el propio aviso.
const avisoAccion = " Pulsa aquí para abrir el panel."

func backupNotice(prev *status.View, next status.View, opt noticeOptions) (title, body string, ok bool) {
	if prev == nil || next.LastBackup == nil {
		return "", "", false
	}
	if prev.LastBackup != nil && prev.LastBackup.FinishedAt == next.LastBackup.FinishedAt {
		return "", "", false
	}
	switch next.LastBackup.Status {
	case "failed":
		const titulo = "No se ha podido hacer la copia"
		if !next.Destination.Connected {
			if next.Destination.Kind == "removable" {
				return titulo, "El disco USB no está conectado. Conéctalo y vuelve a intentarlo." + avisoAccion, true
			}
			return titulo, "El destino de las copias no está disponible. Comprueba que está conectado." + avisoAccion, true
		}
		return titulo, limitar(fallback(next.LastBackup.Message, "Abre el panel para ver qué ha pasado."), 170) + avisoAccion, true
	case "warning":
		if !opt.warnings {
			return "", "", false
		}
		return "La copia terminó con avisos", limitar(fallback(next.LastBackup.Message, "Algunos archivos no se pudieron copiar."), 170) + avisoAccion, true
	case "success":
		if !opt.success {
			return "", "", false
		}
		return "Copia correcta", "Tus archivos están protegidos.", true
	default:
		return "", "", false
	}
}

// limitar recorta un texto largo: el aviso de Windows admite unos 250 caracteres.
func limitar(texto string, max int) string {
	runas := []rune(texto)
	if len(runas) <= max {
		return texto
	}
	return string(runas[:max-1]) + "…"
}

func fallback(message, otherwise string) string {
	if message == "" {
		return otherwise
	}
	return message
}

func tooltip(v status.View, online bool) string {
	title, detail := menuLines(v, online, time.Now())
	if detail == "" {
		return title
	}
	return title + " · " + detail
}

func menuLines(v status.View, online bool, now time.Time) (string, string) {
	if !online {
		return "Servicio parado", "Las copias no se están haciendo"
	}
	title := "Sin información"
	switch {
	case v.Activity == status.ActivityBackup || v.Activity == status.ActivityRestore || v.Level == status.Blue:
		title = "Copiando"
	case v.Activity == status.ActivityVerify:
		title = "Comprobando las copias"
	case v.Level == status.Green:
		title = "Copias al día"
	case v.Level == status.Yellow:
		title = "Con avisos"
	case v.Level == status.Red:
		title = "Falla o sin copia"
	}
	var partes []string
	if v.LastBackup != nil && v.LastBackup.FinishedAt != "" {
		partes = append(partes, "Última: "+cuandoCorto(v.LastBackup.FinishedAt, now))
	}
	if v.NextBackup != "" && v.Activity == "" {
		partes = append(partes, "Próxima: "+cuandoCorto(v.NextBackup, now))
	}
	if len(partes) == 0 {
		return title, "Todavía no hay ninguna copia"
	}
	if line := cloudLine(v.Cloud); line != "" {
		partes = append(partes, line)
	}
	return title, strings.Join(partes, " · ")
}

// cloudLine es el estado de la conexión con la plataforma web, en pocas palabras. Vacío si no
// está configurada: así quien no la usa no ve nada nuevo.
func cloudLine(c *status.Cloud) string {
	if c == nil {
		return ""
	}
	names := map[string]string{
		"connected":       "conectada",
		"offline":         "sin conexión",
		"license_expired": "licencia caducada",
		"license_blocked": "licencia suspendida",
		"seat_limit":      "límite de equipos",
		"revoked":         "equipo desactivado",
		"needs_enroll":    "hay que volver a conectar",
		"not_enrolled":    "sin conectar",
	}
	if name, ok := names[c.State]; ok {
		return "Plataforma: " + name
	}
	return ""
}

func cuandoCorto(iso string, now time.Time) string {
	when, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	when = when.In(time.Local)
	now = now.In(time.Local)
	hora := when.Format("15:04")
	y1, m1, d1 := when.Date()
	y2, m2, d2 := now.Date()
	a := time.Date(y1, m1, d1, 0, 0, 0, 0, time.Local)
	b := time.Date(y2, m2, d2, 0, 0, 0, 0, time.Local)
	dias := int(b.Sub(a).Hours() / 24)
	switch {
	case dias == 0:
		return "hoy " + hora
	case dias == 1:
		return "ayer " + hora
	case dias == -1:
		return "mañana " + hora
	default:
		return strconv.Itoa(when.Day()) + " " + mesCorto(when.Month()) + ", " + hora
	}
}

func mesCorto(m time.Month) string {
	nombres := [...]string{"", "ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}
	if m < 1 || m > 12 {
		return ""
	}
	return nombres[m]
}
