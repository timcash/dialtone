package test_v2

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
)

type PixelColor struct {
	R uint8
	G uint8
	B uint8
	A uint8
}

func ReadPNGPixel(path string, x int, y int) (PixelColor, error) {
	f, err := os.Open(path)
	if err != nil {
		return PixelColor{}, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return PixelColor{}, err
	}

	bounds := img.Bounds()
	if x < bounds.Min.X || x >= bounds.Max.X || y < bounds.Min.Y || y >= bounds.Max.Y {
		return PixelColor{}, fmt.Errorf("pixel (%d,%d) out of bounds %s", x, y, bounds.String())
	}

	r16, g16, b16, a16 := img.At(x, y).RGBA()
	return PixelColor{
		R: uint8(r16 >> 8),
		G: uint8(g16 >> 8),
		B: uint8(b16 >> 8),
		A: uint8(a16 >> 8),
	}, nil
}

func AssertPNGPixelColorWithinTolerance(path string, x int, y int, expected PixelColor, tolerance uint8) error {
	actual, err := ReadPNGPixel(path, x, y)
	if err != nil {
		return err
	}

	if !withinTolerance(actual.R, expected.R, tolerance) ||
		!withinTolerance(actual.G, expected.G, tolerance) ||
		!withinTolerance(actual.B, expected.B, tolerance) ||
		!withinTolerance(actual.A, expected.A, tolerance) {
		return fmt.Errorf(
			"pixel mismatch at (%d,%d): got rgba(%d,%d,%d,%d), expected rgba(%d,%d,%d,%d) ±%d",
			x,
			y,
			actual.R,
			actual.G,
			actual.B,
			actual.A,
			expected.R,
			expected.G,
			expected.B,
			expected.A,
			tolerance,
		)
	}

	return nil
}

func withinTolerance(actual uint8, expected uint8, tolerance uint8) bool {
	diff := int(actual) - int(expected)
	if diff < 0 {
		diff = -diff
	}
	return diff <= int(tolerance)
}
