package main

import (
	"encoding/binary"
	"image"
	"image/color"
)

func circleICO(c color.NRGBA) []byte {
	const n = 32
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			dx := float64(x - 15)
			dy := float64(y - 15)
			if dx*dx+dy*dy <= 13*13 {
				img.SetNRGBA(x, y, c)
			}
		}
	}
	return encodeICO(img)
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
