package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// icon renders a small monochrome "picture/mountain" glyph suitable
// for a menu bar/tray icon: a black shape with alpha transparency, so
// macOS can treat it as a template image (auto-adapts to light/dark
// menu bars) and other platforms render it as a plain black-on-
// transparent icon.
func icon() []byte {
	const size = 22
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	black := color.RGBA{A: 255}

	// Frame border.
	for x := 2; x < size-2; x++ {
		img.SetRGBA(x, 3, black)
		img.SetRGBA(x, size-4, black)
	}
	for y := 3; y < size-3; y++ {
		img.SetRGBA(2, y, black)
		img.SetRGBA(size-3, y, black)
	}

	// Sun.
	img.SetRGBA(6, 7, black)

	// Mountain silhouette (simple triangle-ish zigzag).
	peaks := []struct{ x, yTop int }{
		{4, 14}, {6, 11}, {8, 14}, {10, 9}, {12, 14},
		{14, 12}, {16, 14}, {18, 10},
	}
	for i := 0; i < len(peaks)-1; i++ {
		drawLine(img, peaks[i].x, peaks[i].yTop, peaks[i+1].x, peaks[i+1].yTop, black)
	}
	for x := 4; x <= 18; x++ {
		img.SetRGBA(x, size-5, black)
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// drawLine draws a simple straight line using Bresenham's algorithm.
func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		img.SetRGBA(x0, y0, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
