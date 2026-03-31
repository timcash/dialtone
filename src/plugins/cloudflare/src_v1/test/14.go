package main

import (
	"fmt"
	"time"
)

func Run14ThreeSectionValidation() error {
	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := navigateToSection(session, "three"); err != nil {
		return err
	}
	if err := session.WaitForAriaLabel("Three Canvas", 5*time.Second); err != nil {
		return err
	}

	if err := session.Evaluate(`
    (() => {
      const c = document.querySelector("[aria-label='Three Canvas']");
      if (!c) return;
      c.dispatchEvent(new WheelEvent('wheel', { deltaY: 120 }));
    })();
  `, nil); err != nil {
		return err
	}

	if err := session.WaitForAriaLabelAttrEquals("Three Canvas", "data-wheel-count", "1", 3*time.Second); err != nil {
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
	if err := session.Evaluate(`
		(() => {
			const api = window.cloudflareThreeDebug;
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
	`, &target); err != nil {
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
	if err := session.Evaluate(fmt.Sprintf(`
		(() => {
			const api = window.cloudflareThreeDebug;
			if (!api || typeof api.touchProjected !== 'function') return false;
			return api.touchProjected('%s');
		})()
	`, selectedID), &hitOK); err != nil {
		return err
	}
	if !hitOK {
		return fmt.Errorf("three projected touch-test did not return %s", selectedID)
	}

	if err := session.WaitForAriaLabelAttrEquals("Three Canvas", "data-selected-cube", selectedID, 3*time.Second); err != nil {
		return err
	}
	if !session.HasConsoleMessage(fmt.Sprintf("[Three #%s] touch cube: %s", cloudflareSectionID("three"), selectedID)) {
		fmt.Printf("[TEST] three touch log not cached locally for %s; relying on selected-cube state instead\n", selectedID)
	}

	shot, err := screenshotPath("test_step_4.png")
	if err != nil {
		return err
	}
	if err := session.CaptureScreenshot(shot); err != nil {
		return err
	}

	fmt.Printf("[TEST] Three touch-test selected %s\n", selectedID)
	fmt.Printf("[TEST] Three screenshot captured: %s\n", shot)
	return nil
}
