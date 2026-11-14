package main

import (
	"bytes"
	"embed"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
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

// trayICO devuelve el icono de un estado de px por px píxeles, en el formato que espera iconHandle.
// Los ficheros de iconos/ traen el escudo con un margen grande alrededor, y en 16 px quedaba
// diminuto al lado de los demás iconos de la bandeja. Se parte de la imagen más grande, se recorta
// el margen transparente y se reduce el escudo para que llene el cuadrado.
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
		if w > bestW {
			bestW = w
			bestSize = int(binary.LittleEndian.Uint32(entry[8:]))
			bestOff = int(binary.LittleEndian.Uint32(entry[12:]))
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
	src := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(src, src.Bounds(), img, b.Min, draw.Src)
	return encodeICO(fitToSquare(src, px)), nil
}

// fitToSquare recorta el margen transparente de src y lo ajusta, centrado y sin deformar,
// a un cuadrado de px por px con un margen mínimo.
func fitToSquare(src *image.NRGBA, px int) *image.NRGBA {
	box := contentBounds(src)
	out := image.NewNRGBA(image.Rect(0, 0, px, px))
	margin := 0
	if px >= 24 {
		margin = 1
	}
	avail := px - 2*margin
	bw, bh := box.Dx(), box.Dy()
	if bw <= 0 || bh <= 0 || avail <= 0 {
		return out
	}
	scale := float64(avail) / float64(bw)
	if s := float64(avail) / float64(bh); s < scale {
		scale = s
	}
	dw := int(float64(bw)*scale + 0.5)
	dh := int(float64(bh)*scale + 0.5)
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	offX := (px - dw) / 2
	offY := (px - dh) / 2
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			// Rectángulo de la imagen original que cae en este píxel (promedio ponderado por la transparencia).
			x0 := float64(box.Min.X) + float64(x)*float64(bw)/float64(dw)
			x1 := float64(box.Min.X) + float64(x+1)*float64(bw)/float64(dw)
			y0 := float64(box.Min.Y) + float64(y)*float64(bh)/float64(dh)
			y1 := float64(box.Min.Y) + float64(y+1)*float64(bh)/float64(dh)
			out.SetNRGBA(offX+x, offY+y, areaAverage(src, x0, y0, x1, y1))
		}
	}
	return out
}

// contentBounds es el rectángulo mínimo que contiene los píxeles con algo de opacidad.
func contentBounds(img *image.NRGBA) image.Rectangle {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.NRGBAAt(x, y).A > 8 {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
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
	if maxX < minX || maxY < minY {
		return image.Rectangle{}
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

// areaAverage promedia los píxeles de src dentro de [x0,x1)×[y0,y1), con alfa premultiplicado
// para que los bordes transparentes no ensucien el color.
func areaAverage(src *image.NRGBA, x0, y0, x1, y1 float64) color.NRGBA {
	var r, g, b, a, weight float64
	for y := int(y0); float64(y) < y1; y++ {
		wy := overlap(float64(y), float64(y+1), y0, y1)
		for x := int(x0); float64(x) < x1; x++ {
			wx := overlap(float64(x), float64(x+1), x0, x1)
			w := wx * wy
			if w <= 0 || !image.Pt(x, y).In(src.Bounds()) {
				continue
			}
			c := src.NRGBAAt(x, y)
			alpha := float64(c.A) / 255
			r += float64(c.R) * alpha * w
			g += float64(c.G) * alpha * w
			b += float64(c.B) * alpha * w
			a += alpha * w
			weight += w
		}
	}
	if weight == 0 || a == 0 {
		return color.NRGBA{}
	}
	return color.NRGBA{
		R: uint8(r/a + 0.5),
		G: uint8(g/a + 0.5),
		B: uint8(b/a + 0.5),
		A: uint8(a/weight*255 + 0.5),
	}
}

func overlap(a0, a1, b0, b1 float64) float64 {
	lo, hi := a0, a1
	if b0 > lo {
		lo = b0
	}
	if b1 < hi {
		hi = b1
	}
	if hi <= lo {
		return 0
	}
	return hi - lo
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
