//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// promoteTrayIcon marca el icono como visible en la barra (fuera del desbordamiento ^).
func promoteTrayIcon() {
	self, err := os.Executable()
	if err != nil {
		return
	}
	self, err = filepath.Abs(self)
	if err != nil {
		return
	}
	self = strings.ToLower(filepath.Clean(self))

	key, err := registry.OpenKey(registry.CURRENT_USER, `Control Panel\NotifyIconSettings`, registry.READ)
	if err != nil {
		return
	}
	defer key.Close()
	names, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return
	}
	for _, name := range names {
		sub, err := registry.OpenKey(key, name, registry.QUERY_VALUE|registry.SET_VALUE)
		if err != nil {
			continue
		}
		exe, _, err := sub.GetStringValue("ExecutablePath")
		if err != nil {
			sub.Close()
			continue
		}
		exe = strings.ToLower(filepath.Clean(exe))
		if exe == self || strings.HasSuffix(exe, `\tray.exe`) && strings.Contains(exe, `\backupamg\`) {
			_ = sub.SetDWordValue("IsPromoted", 1)
		}
		sub.Close()
	}
}
