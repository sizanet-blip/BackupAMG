//go:build windows

package main

import (
	"image/color"
	"os"
	"os/exec"
	"time"

	"backup-agent/internal/panel"
	"backup-agent/internal/status"
	"fyne.io/systray"
	"golang.org/x/sys/windows"
)

func main() {
	if alreadyRunning() {
		os.Exit(0)
	}
	systray.Run(onReady, func() {})
}

func alreadyRunning() bool {
	name, err := windows.UTF16PtrFromString(`Local\BackupAgentTray`)
	if err != nil {
		return false
	}
	_, err = windows.CreateMutex(nil, false, name)
	return err == windows.ERROR_ALREADY_EXISTS
}

func onReady() {
	icons := map[string][]byte{
		status.Green:  circleICO(color.NRGBA{R: 23, G: 121, B: 74, A: 255}),
		status.Yellow: circleICO(color.NRGBA{R: 224, G: 161, B: 0, A: 255}),
		status.Red:    circleICO(color.NRGBA{R: 168, G: 36, B: 26, A: 255}),
		status.Blue:   circleICO(color.NRGBA{R: 35, G: 80, B: 216, A: 255}),
		"gray":        circleICO(color.NRGBA{R: 138, G: 144, B: 155, A: 255}),
	}
	systray.SetIcon(icons["gray"])
	systray.SetTooltip("Servicio parado")

	stateItem := systray.AddMenuItem("Servicio parado", "Las copias no se están haciendo")
	stateItem.Disable()
	openItem := systray.AddMenuItem("Abrir panel", "")
	copyItem := systray.AddMenuItem("Copiar ahora", "")
	historyItem := systray.AddMenuItem("Ver historial", "")
	restoreItem := systray.AddMenuItem("Restaurar archivos", "")
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Ocultar el icono", "Las copias siguen funcionando")
	systray.AddSeparator()
	legend := systray.AddMenuItem("Colores del icono", "")
	legend.Disable()
	okItem := systray.AddMenuItem("Al día", "")
	warnItem := systray.AddMenuItem("Con avisos", "")
	badItem := systray.AddMenuItem("Falla o sin copia", "")
	busyItem := systray.AddMenuItem("Copiando", "")
	offItem := systray.AddMenuItem("Servicio parado", "")
	for _, item := range []*systray.MenuItem{okItem, warnItem, badItem, busyItem, offItem} {
		item.Disable()
	}
	okItem.SetIcon(icons[status.Green])
	warnItem.SetIcon(icons[status.Yellow])
	badItem.SetIcon(icons[status.Red])
	busyItem.SetIcon(icons[status.Blue])
	offItem.SetIcon(icons["gray"])

	var prev *status.View
	refresh := func() {
		view, err := panel.RequestStatus()
		if err != nil {
			systray.SetIcon(icons["gray"])
			title, detail := menuLines(status.View{}, false, time.Now())
			systray.SetTooltip(title + " · " + detail)
			stateItem.SetTitle(title)
			stateItem.SetTooltip(detail)
			openItem.Disable()
			copyItem.Disable()
			historyItem.Disable()
			restoreItem.Disable()
			prev = nil
			return
		}
		level := view.Level
		if view.Activity == status.ActivityBackup || view.Activity == status.ActivityVerify || view.Activity == status.ActivityRestore {
			level = status.Blue
		}
		icon := icons[level]
		if icon == nil {
			icon = icons["gray"]
		}
		systray.SetIcon(icon)
		title, detail := menuLines(view, true, time.Now())
		tip := title
		if detail != "" {
			tip += " · " + detail
		}
		systray.SetTooltip(tip)
		stateItem.SetTitle(title + " · " + detail)
		stateItem.SetTooltip(detail)
		openItem.Enable()
		historyItem.Enable()
		restoreItem.Enable()
		if level == status.Blue {
			copyItem.Disable()
		} else {
			copyItem.Enable()
		}
		if noteTitle, body, ok := backupNotice(prev, view, noticeOptions{warnings: true}); ok {
			pushToast(noteTitle, body)
		}
		prev = &view
	}
	refresh()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			refresh()
		case <-openItem.ClickedCh:
			openPanel("")
		case <-historyItem.ClickedCh:
			openPanel("historial")
		case <-restoreItem.ClickedCh:
			openPanel("restaurar")
		case <-copyItem.ClickedCh:
			if err := panel.RequestBackup(); err != nil {
				pushToast("No se pudo iniciar la copia", err.Error())
			}
			refresh()
		case <-quitItem.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func openPanel(next string) {
	url, _, _, err := panel.RequestToken()
	if err != nil {
		pushToast("No se pudo abrir el panel", "El agente no está en marcha.")
		return
	}
	if next == "historial" || next == "restaurar" {
		url += "&next=" + next
	}
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
