// Command brandicon draws Vibe's terminal prompt icon into
// cmd/vibe/icon-art.png, the art tools/icongen puts on the macOS icon grid.
//
//	go run ./tools/brandicon
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"

	xdraw "golang.org/x/image/draw"
)

const (
	out    = 1254 // the size of the art tools/icongen reads
	scale  = 3
	bg     = 0x121218
	green  = 0x7ee787
	stroke = 0.058 // half the chevron's width, of the art's width
)

func main() {
	w := out * scale
	img := image.NewRGBA(image.Rect(0, 0, w, w))
	// A full-bleed tile: icongen crops its border and clips the rest to the
	// macOS squircle, and the welcome screen clips it the same way.
	segment1 := [4]float64{0.27, 0.31, 0.455, 0.5}
	segment2 := [4]float64{0.455, 0.5, 0.27, 0.69}
	r := stroke * float64(w)
	for y := 0; y < w; y++ {
		fy := (float64(y) + 0.5) / float64(w)
		for x := 0; x < w; x++ {
			fx := (float64(x) + 0.5) / float64(w)
			px := color.RGBA{R: bg >> 16, G: (bg >> 8) & 0xff, B: bg & 0xff, A: 255}
			// A soft green wash behind the prompt, so the tile is not flat.
			d := math.Hypot((fx-0.47)*1.1, (fy - 0.5))
			if d < 0.36 {
				t := 1 - d/0.36
				px = mix(px, color.RGBA{0x1e, 0x3a, 0x2a, 255}, 0.22*t*t)
			}
			cov := math.Max(distance(fx*float64(w), fy*float64(w), segment1, r),
				distance(fx*float64(w), fy*float64(w), segment2, r))
			// The underscore: a rounded bar from 55% to 73%, at 64%-72%.
			cov = math.Max(cov, roundBar(fx, fy, 0.55, 0.64, 0.73, 0.72, 0.045))
			if cov > 0 {
				g := color.RGBA{R: green >> 16, G: (green >> 8) & 0xff, B: green & 0xff, A: 255}
				px = mix(px, g, math.Min(1, cov))
			}
			img.SetRGBA(x, y, px)
		}
	}
	small := image.NewRGBA(image.Rect(0, 0, out, out))
	xdraw.CatmullRom.Scale(small, small.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	f, err := os.Create("cmd/vibe/icon-art.png")
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(f, small); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}

// distance is 1 inside a capsule around the segment, falling off over about
// a pixel, and 0 outside it.
func distance(x, y float64, s [4]float64, r float64) float64 {
	dx, dy := s[2]-s[0], s[3]-s[1]
	l := dx*dx + dy*dy
	t := ((x/float64(out*scale)-s[0])*dx + (y/float64(out*scale)-s[1])*dy) / l
	t = math.Max(0, math.Min(1, t))
	px, py := (s[0]+t*dx)*float64(out*scale), (s[1]+t*dy)*float64(out*scale)
	d := math.Hypot(x-px, y-py)
	if d <= r {
		return 1
	}
	return math.Max(0, 1-(d-r)/float64(scale))
}

// roundBar is 1 inside the rounded bar, 0 outside, with a one-pixel edge.
func roundBar(x, y, x0, y0, x1, y1, radius float64) float64 {
	// Inset the rectangle by the radius, then measure the distance to it.
	ix0, iy0, ix1, iy1 := x0+radius, y0+radius, x1-radius, y1-radius
	cx := math.Max(ix0, math.Min(ix1, x))
	cy := math.Max(iy0, math.Min(iy1, y))
	d := math.Hypot(x-cx, y-cy)
	if d <= radius {
		return 1
	}
	return math.Max(0, 1-(d-radius)/(float64(scale)/float64(out*scale)))
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	m := func(x, y uint8) uint8 { return uint8(float64(x)*(1-t) + float64(y)*t + 0.5) }
	return color.RGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 255}
}
