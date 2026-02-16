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

	type points struct {
		Left  point `json:"left"`
		Right point `json:"right"`
	}

	var target points
	if err := session.Run(chromedp.Evaluate(`
		(() => {
			const api = window.robotThreeDebug;
			if (!api || typeof api.getProjectedPoint !== 'function') {
				return {
					left: { ok: false, x: 0, y: 0 },
					right: { ok: false, x: 0, y: 0 }
				};
			}
			return {
				left: api.getProjectedPoint('cube_left'),
				right: api.getProjectedPoint('cube_right')
			};
		})()
	`, &target)); err != nil {
		return err
	}
	if !target.Left.Ok && !target.Right.Ok {
		return fmt.Errorf("three debug projected point api unavailable")
	}

	viewportW := float64(testViewportWidth)
	viewportH := float64(testViewportHeight)
	inBounds := func(p point) bool {
		return p.Ok && p.X >= 0 && p.Y >= 0 && p.X < viewportW && p.Y < viewportH
	}
	selectedID := "cube_left"
	selected := target.Left
	if !inBounds(selected) && inBounds(target.Right) {
		selectedID = "cube_right"
		selected = target.Right
	}
	if !inBounds(selected) {
		return fmt.Errorf("no projected cube point in viewport: left=(%.1f,%.1f) right=(%.1f,%.1f)", target.Left.X, target.Left.Y, target.Right.X, target.Right.Y)
	}

	var hitOK bool
	if err := session.Run(chromedp.Evaluate(fmt.Sprintf(`
		(() => {
			const api = window.robotThreeDebug;
			if (!api || typeof api.touchProjected !== 'function') return false;
			return api.touchProjected('%s');
		})()
	`, selectedID), &hitOK)); err != nil {
		return err
	}
	if !hitOK {
		return fmt.Errorf("three projected touch-test did not return %s", selectedID)
	}

	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("Three Canvas", "data-selected-cube", selectedID, 3*time.Second)); err != nil {
		return err
	}
	if !session.HasConsoleMessage(fmt.Sprintf("[Three #three] touch cube: %s", selectedID)) {
		return fmt.Errorf("missing three touch hit-test log for %s", selectedID)
	}

	shot := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v1", "screenshots", "test_step_4.png")
	if err := session.CaptureScreenshot(shot); err != nil {
		return err
	}

	px := int(math.Round(selected.X))
	py := int(math.Round(selected.Y))
	if err := test_v2.AssertPNGPixelColorWithinTolerance(shot, px, py, test_v2.PixelColor{
		R: 70,
		G: 120,
		B: 220,
		A: 255,
	}, 100); err != nil {
		return fmt.Errorf("three highlight pixel check failed: %w", err)
	}

	fmt.Printf("[TEST] Three touch-test selected %s at (%d,%d)\n", selectedID, px, py)
	fmt.Printf("[TEST] Three screenshot pixel check passed at (%d,%d)\n", px, py)
	return nil
}
