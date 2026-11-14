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

func backupNotice(prev *status.View, next status.View, opt noticeOptions) (title, body string, ok bool) {
	if prev == nil || next.LastBackup == nil {
		return "", "", false
	}
	if prev.LastBackup != nil && prev.LastBackup.FinishedAt == next.LastBackup.FinishedAt {
		return "", "", false
	}
	if !next.Destination.Connected {
		return "Destino no conectado", "La copia no encuentra el disco o la carpeta de destino.", true
	}
	switch next.LastBackup.Status {
	case "failed":
		return "La copia ha fallado", fallback(next.LastBackup.Message, "La última copia no se ha completado."), true
	case "warning":
		if !opt.warnings {
			return "", "", false
		}
		return "La copia terminó con avisos", fallback(next.LastBackup.Message, "Algunos ficheros no se pudieron leer."), true
	case "success":
		if !opt.success {
			return "", "", false
		}
		return "Copia correcta", fallback(next.LastBackup.Message, "La copia se ha completado."), true
	default:
		return "", "", false
	}
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
	return title, strings.Join(partes, " · ")
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
