package main

import (
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

	shot := filepath.Join(repoRoot, "src", "plugins", "template", "src_v3", "screenshots", "test_step_4.png")
	return session.CaptureScreenshot(shot)
}
