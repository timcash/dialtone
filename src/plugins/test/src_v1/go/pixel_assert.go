package test

import (
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"os"
)

func AssertPNGPixelColorWithinTolerance(path string, x, y int, expected color.RGBA, tolerance int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	actual := img.At(x, y)
	r, g, b, a := actual.RGBA()
	
	// RGBA() returns values in range [0, 65535]
	ar := uint8(r >> 8)
	ag := uint8(g >> 8)
	ab := uint8(b >> 8)
	aa := uint8(a >> 8)

	diff := func(a, b uint8) int {
		if a > b {
			return int(a - b)
		}
		return int(b - a)
	}

	if diff(ar, expected.R) > tolerance || diff(ag, expected.G) > tolerance || diff(ab, expected.B) > tolerance || diff(aa, expected.A) > tolerance {
		return fmt.Errorf("pixel at (%d, %d) color %v is out of tolerance from %v", x, y, color.RGBA{ar, ag, ab, aa}, expected)
	}

	return nil
}
