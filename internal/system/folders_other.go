//go:build !windows

package system

import "context"

type Shortcut struct {
	ID   string `json:"id"`
	User string `json:"user"`
	Path string `json:"path"`
}

type Dir struct {
	Name string `json:"name"`
	Path string `json:"path"`
	// Solo en las unidades, y solo si la unidad responde a tiempo.
	Label      string `json:"label,omitempty"`
	FreeBytes  int64  `json:"free_bytes,omitempty"`
	TotalBytes int64  `json:"total_bytes,omitempty"`
}

func Shortcuts() []Shortcut { return nil }

func ListDirs(string) ([]Dir, error) { return nil, errFolder("no disponible") }

func EstimateBytes(context.Context, []string) (int64, bool) { return 0, false }

type errFolder string

func (e errFolder) Error() string { return string(e) }
