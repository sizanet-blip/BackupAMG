//go:build windows

package main

import (
	"os/exec"
	"strings"
	"syscall"
)

func pushToast(title, body string) {
	script := `[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml('` + toastXML(title, body) + `')
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
$notifier = [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe')
$notifier.Show($toast)`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()
}

func toastXML(title, body string) string {
	return `<toast><visual><binding template="ToastGeneric"><text>` + xmlEscape(title) + `</text><text>` + xmlEscape(body) + `</text></binding></visual></toast>`
}

func xmlEscape(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, "'", "&apos;")
	value = strings.ReplaceAll(value, `"`, "&quot;")
	return value
}
