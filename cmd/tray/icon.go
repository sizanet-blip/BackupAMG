package main

import (
	"bytes"
	"embed"
	"encoding/binary"
	"errors"
	"image"
	"image/draw"
	"image/png"
)

//go:embed iconos/*.ico
var trayIcons embed.FS

// Nombres de los iconos de la bandeja, uno por estado.
const (
	iconOK    = "tray-ok.ico"
	iconWarn  = "tray-warn.ico"
	iconError = "tray-error.ico"
	iconBusy  = "tray-busy.ico"
	iconOff   = "tray-off.ico"
)

// trayICO devuelve el icono de un estado en el tamaño del fichero que más se acerca a px
// (el menor que no sea más pequeño que px, o el mayor si no hay). Sale en el formato que
// espera iconHandle. Los ficheros de iconos/ vienen en PNG dentro del .ico.
func trayICO(name string, px int) ([]byte, error) {
	raw, err := trayIcons.ReadFile("iconos/" + name)
	if err != nil {
		return nil, err
	}
	if len(raw) < 22 || binary.LittleEndian.Uint16(raw[2:]) != 1 {
		return nil, errors.New("el fichero no es un icono")
	}
	count := int(binary.LittleEndian.Uint16(raw[4:]))
	bestW, bestOff, bestSize := 0, 0, 0
	for i := 0; i < count; i++ {
		entry := raw[6+16*i:]
		w := int(entry[0])
		if w == 0 {
			w = 256
		}
		size := int(binary.LittleEndian.Uint32(entry[8:]))
		off := int(binary.LittleEndian.Uint32(entry[12:]))
		better := bestW == 0 ||
			(bestW < px && w > bestW) ||
			(w >= px && w < bestW)
		if better {
			bestW, bestOff, bestSize = w, off, size
		}
	}
	if bestW == 0 || bestOff+bestSize > len(raw) {
		return nil, errors.New("el icono no tiene imágenes")
	}
	img, err := png.Decode(bytes.NewReader(raw[bestOff : bestOff+bestSize]))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	nrgba := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(nrgba, nrgba.Bounds(), img, b.Min, draw.Src)
	return encodeICO(nrgba), nil
}

func encodeICO(img *image.NRGBA) []byte {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	xor := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.NRGBAAt(x, h-1-y)
			i := (y*w + x) * 4
			xor[i] = c.B
			xor[i+1] = c.G
			xor[i+2] = c.R
			xor[i+3] = c.A
		}
	}
	rowBytes := ((w + 31) / 32) * 4
	and := make([]byte, rowBytes*h)
	header := make([]byte, 40)
	binary.LittleEndian.PutUint32(header[0:], 40)
	binary.LittleEndian.PutUint32(header[4:], uint32(w))
	binary.LittleEndian.PutUint32(header[8:], uint32(h*2))
	binary.LittleEndian.PutUint16(header[12:], 1)
	binary.LittleEndian.PutUint16(header[14:], 32)
	imageSize := uint32(len(header) + len(xor) + len(and))
	dir := make([]byte, 6)
	binary.LittleEndian.PutUint16(dir[2:], 1)
	binary.LittleEndian.PutUint16(dir[4:], 1)
	entry := make([]byte, 16)
	entry[0] = byte(w)
	entry[1] = byte(h)
	binary.LittleEndian.PutUint16(entry[4:], 1)
	binary.LittleEndian.PutUint16(entry[6:], 32)
	binary.LittleEndian.PutUint32(entry[8:], imageSize)
	binary.LittleEndian.PutUint32(entry[12:], 22)
	out := make([]byte, 0, 22+len(header)+len(xor)+len(and))
	out = append(out, dir...)
	out = append(out, entry...)
	out = append(out, header...)
	out = append(out, xor...)
	out = append(out, and...)
	return out
}
