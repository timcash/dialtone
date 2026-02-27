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
	if err := downscalePNGHalf(path); err != nil {
		return err
	}
	return downscalePNGHalf(path)
}

func resolvePaths() (uiv1.Paths, error) {
	return uiv1.ResolvePaths("")
}

func downscalePNGHalf(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return err
	}
	b := src.Bounds()
	w := b.Dx() / 2
	h := b.Dy() / 2
	if w < 1 || h < 1 {
		return nil
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		sy := b.Min.Y + y*2
		for x := 0; x < w; x++ {
			sx := b.Min.X + x*2
			c := color.RGBAModel.Convert(src.At(sx, sy)).(color.RGBA)
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
