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

func Run14ThreeSectionValidation() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := session.Run(test_v2.NavigateToSection("three", "Three Section")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabel("Three Canvas")); err != nil {
		return err
	}

	if err := session.Run(chromedp.Evaluate(`
    (() => {
      const c = document.querySelector("[aria-label='Three Canvas']");
      if (!c) return;
      c.dispatchEvent(new WheelEvent('wheel', { deltaY: 120 }));
    })();
  `, nil)); err != nil {
		return err
	}

	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("Three Canvas", "data-wheel-count", "1", 3*time.Second)); err != nil {
		return err
	}

	type point struct {
		Ok bool    `json:"ok"`
		X  float64 `json:"x"`
		Y  float64 `json:"y"`
	}

	var target point
	if err := session.Run(chromedp.Evaluate(`
		(() => {
			const api = window.templateThreeDebug;
			if (!api || typeof api.getProjectedPoint !== 'function') {
				return { ok: false, x: 0, y: 0 };
			}
			return api.getProjectedPoint('cube_left');
		})()
	`, &target)); err != nil {
		return err
	}
	if !target.Ok {
		return fmt.Errorf("three debug projected point api unavailable")
	}

	if err := session.Run(chromedp.Evaluate(fmt.Sprintf(`
		(() => {
			const c = document.querySelector("[aria-label='Three Canvas']");
			if (!c) return;
			const evt = new MouseEvent('mousemove', {
				clientX: %f,
				clientY: %f,
				bubbles: true
			});
			c.dispatchEvent(evt);
		})()
	`, target.X, target.Y), nil)); err != nil {
		return err
	}

	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("Three Canvas", "data-hovered-cube", "cube_left", 3*time.Second)); err != nil {
		return err
	}
	if !session.HasConsoleMessage("[Three #three] hover cube: cube_left") {
		return fmt.Errorf("missing three hover hit-test log for cube_left")
	}

	shot := filepath.Join(repoRoot, "src", "plugins", "template", "src_v3", "screenshots", "test_step_4.png")
	if err := session.CaptureScreenshot(shot); err != nil {
		return err
	}

	px := int(math.Round(target.X))
	py := int(math.Round(target.Y))
	if err := test_v2.AssertPNGPixelColorWithinTolerance(shot, px, py, test_v2.PixelColor{
		R: 70,
		G: 120,
		B: 220,
		A: 255,
	}, 100); err != nil {
		return fmt.Errorf("three highlight pixel check failed: %w", err)
	}

	fmt.Printf("[TEST] Three hit-test hovered cube_left at (%d,%d)\n", px, py)
	fmt.Printf("[TEST] Three screenshot pixel check passed at (%d,%d)\n", px, py)
	return nil
}
