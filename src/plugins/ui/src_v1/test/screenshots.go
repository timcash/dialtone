package test

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	uiv1 "dialtone/dev/plugins/ui/src_v1/go"
)

const screenshotsDirRel = "plugins/ui/src_v1/test/screenshots"

func ScreenshotPath(filename string) (string, error) {
	paths, err := resolvePaths()
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(filename)
	if name == "" {
		return "", fmt.Errorf("screenshot filename is empty")
	}
	dir := filepath.Join(paths.Runtime.SrcRoot, screenshotsDirRel)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

func CaptureScreenshot(sc *StepContext, filename string) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	path, err := ScreenshotPath(filename)
	if err != nil {
		return err
	}
	if err := b.CaptureScreenshot(path); err != nil {
		return err
	}
	if err := ResizeScreenshotHalf(path); err != nil {
		return err
	}
	return nil
}

func resolvePaths() (uiv1.Paths, error) {
	return uiv1.ResolvePaths("")
}

// ResizeScreenshotHalf rewrites a PNG on disk to 50% width/height.
func ResizeScreenshotHalf(path string) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()

	src, err := png.Decode(in)
	if err != nil {
		return err
	}
	b := src.Bounds()
	outW := b.Dx() / 2
	outH := b.Dy() / 2
	if outW < 1 || outH < 1 {
		return nil
	}

	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))
	for y := 0; y < outH; y++ {
		srcY := b.Min.Y + (y * 2)
		for x := 0; x < outW; x++ {
			srcX := b.Min.X + (x * 2)
			c := color.RGBAModel.Convert(src.At(srcX, srcY)).(color.RGBA)
			dst.SetRGBA(x, y, c)
		}
	}

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	return png.Encode(out, dst)
}
