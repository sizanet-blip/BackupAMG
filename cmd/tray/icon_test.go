package main

import (
	"image/color"
	"testing"
)

func TestCircleIconHasTheICOHeader(t *testing.T) {
	raw := circleICO(color.NRGBA{R: 47, G: 158, B: 90, A: 255})
	if len(raw) < 22 || raw[2] != 1 || raw[4] != 1 {
		t.Fatalf("cabecera %v", raw[:6])
	}
}
