package main

import (
	"encoding/binary"
	"testing"
)

// visibleBox devuelve el rectángulo que ocupan los píxeles visibles del icono ya preparado.
func visibleBox(raw []byte, px int) (minX, minY, maxX, maxY int) {
	minX, minY, maxX, maxY = px, px, -1, -1
	for row := 0; row < px; row++ {
		for col := 0; col < px; col++ {
			// Los píxeles van de abajo arriba, en BGRA, después de la cabecera de 22+40 bytes.
			y := px - 1 - row
			a := raw[62+(row*px+col)*4+3]
			if a > 8 {
				if col < minX {
					minX = col
				}
				if col > maxX {
					maxX = col
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	return
}

func TestTrayIconsExistInEveryStateAndSize(t *testing.T) {
	for _, name := range []string{iconOK, iconWarn, iconError, iconBusy, iconOff} {
		for _, px := range []int{16, 20, 24, 32, 48} {
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
			minX, minY, maxX, maxY := visibleBox(raw, px)
			if maxX < 0 {
				t.Fatalf("%s a %d px está vacío", name, px)
			}
			// El escudo debe llenar el cuadrado: así no se ve más pequeño que los demás iconos.
			width, height := maxX-minX+1, maxY-minY+1
			big := width
			if height > big {
				big = height
			}
			if big < px*9/10 {
				t.Fatalf("%s a %d px: el escudo solo ocupa %dx%d, debería llenar el icono", name, px, width, height)
			}
		}
	}
}
