package main

import (
	"encoding/binary"
	"testing"
)

func TestTrayIconsExistInEveryStateAndSize(t *testing.T) {
	for _, name := range []string{iconOK, iconWarn, iconError, iconBusy, iconOff} {
		for _, px := range []int{16, 20, 24, 32} {
			raw, err := trayICO(name, px)
			if err != nil {
				t.Fatalf("%s a %d px: %v", name, px, err)
			}
			if len(raw) < 62 || raw[2] != 1 || raw[4] != 1 {
				t.Fatalf("%s: cabecera %v", name, raw[:6])
			}
			if got := int(binary.LittleEndian.Uint32(raw[26:])); got != px {
				t.Fatalf("%s: se pidió %d px y salió de %d", name, px, got)
			}
			// Debe haber píxeles visibles: el icono no puede salir vacío.
			visible := 0
			for i := 62 + 3; i < len(raw)-px*((px+31)/32)*4; i += 4 {
				if raw[i] > 0 {
					visible++
				}
			}
			if visible == 0 {
				t.Fatalf("%s a %d px está vacío", name, px)
			}
		}
	}
}

func TestTrayIconPicksTheNearestLargerSize(t *testing.T) {
	raw, err := trayICO(iconOK, 40) // no hay 40: debe usar 48
	if err != nil {
		t.Fatal(err)
	}
	if got := int(binary.LittleEndian.Uint32(raw[26:])); got != 48 {
		t.Fatalf("para 40 px se esperaba 48, salió %d", got)
	}
	big, err := trayICO(iconOK, 512) // más grande que todos: el mayor
	if err != nil {
		t.Fatal(err)
	}
	if got := int(binary.LittleEndian.Uint32(big[26:])); got != 48 {
		t.Fatalf("para 512 px se esperaba el mayor (48), salió %d", got)
	}
}
