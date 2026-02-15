package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	test_v2 "dialtone/cli/src/libs/test_v2"
	"github.com/chromedp/chromedp"
)

func Run02HitTestSectionValidation() error {
	browser, err := ensureSharedBrowser(true)
	if err != nil {
		return err
	}

	type projectedPoint struct {
		OK bool    `json:"ok"`
		X  float64 `json:"x"`
		Y  float64 `json:"y"`
	}
	type viewState struct {
		Point projectedPoint `json:"point"`
		W     float64        `json:"w"`
		H     float64        `json:"h"`
	}
	var state viewState
	if err := browser.Run(chromedp.Tasks{
		chromedp.Navigate("http://127.0.0.1:8080/#hit-test"),
		test_v2.WaitForAriaLabel("Hit Test Canvas"),
		test_v2.WaitForAriaLabelAttrEquals("Hit Test Canvas", "data-wheel-count", "0", 2*time.Second),
		chromedp.Evaluate(`
			(() => {
				const api = window.dagHitTestDebug;
				if (!api || typeof api.getProjectedPoint !== 'function') {
					throw new Error('dagHitTestDebug API unavailable');
				}
				const p = api.getProjectedPoint('cube_left');
				return { point: p, w: window.innerWidth, h: window.innerHeight };
			})()
		`, &state),
	}); err != nil {
		return err
	}

	if !state.Point.OK {
		return fmt.Errorf("getProjectedPoint(cube_left) failed")
	}
	clickX := state.Point.X
	clickY := state.Point.Y
	if clickX < 0 || clickY < 0 || clickX >= state.W || clickY >= state.H {
		return fmt.Errorf(
			"cube projected point is outside viewport: (%.1f,%.1f) not in %.0fx%.0f",
			clickX,
			clickY,
			state.W,
			state.H,
		)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	beforeShot := filepath.Join(repoRoot, "src", "plugins", "dag", "src_v3", "screenshots", "test_step_1_before.png")
	afterShot := filepath.Join(repoRoot, "src", "plugins", "dag", "src_v3", "screenshots", "test_step_1.png")
	if err := browser.CaptureScreenshot(beforeShot); err != nil {
		return fmt.Errorf("capture pre-click screenshot: %w", err)
	}

	var clickOK bool
	if err := browser.Run(chromedp.Tasks{
		chromedp.Evaluate(`
			(() => {
				const api = window.dagHitTestDebug;
				if (!api || typeof api.clickProjected !== 'function') return false;
				return api.clickProjected('cube_left');
			})()
		`, &clickOK),
		test_v2.WaitForAriaLabelAttrEquals("Hit Test Canvas", "data-selected-cube", "cube_left", 2*time.Second),
	}); err != nil {
		return err
	}
	if !clickOK {
		return fmt.Errorf("clickProjected(cube_left) returned false")
	}

	if err := browser.CaptureScreenshot(afterShot); err != nil {
		return fmt.Errorf("capture post-click screenshot: %w", err)
	}

	if err := assertPixelBlueGain(beforeShot, afterShot, int(math.Round(clickX)), int(math.Round(clickY))); err != nil {
		return err
	}

	return nil
}

func assertPixelBlueGain(beforePath, afterPath string, x, y int) error {
	before, err := test_v2.ReadPNGPixel(beforePath, x, y)
	if err != nil {
		return fmt.Errorf("read pre-click pixel: %w", err)
	}
	after, err := test_v2.ReadPNGPixel(afterPath, x, y)
	if err != nil {
		return fmt.Errorf("read post-click pixel: %w", err)
	}

	blueGain := int(after.B) - int(before.B)
	if blueGain < 25 || after.B < after.R+10 || after.B < after.G+10 {
		return fmt.Errorf(
			"pixel color did not shift to blue highlight at (%d,%d): before rgba(%d,%d,%d,%d), after rgba(%d,%d,%d,%d)",
			x, y,
			before.R, before.G, before.B, before.A,
			after.R, after.G, after.B, after.A,
		)
	}

	return nil
}
