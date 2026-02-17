package main

import (
	"os"
	"path/filepath"

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

	// 1. Navigate to 3D Section
	if err := session.Run(test_v2.NavigateToSection("three", "Three Section")); err != nil {
		return err
	}

	// 2. Verify Canvas is visible
	if err := session.Run(chromedp.WaitVisible("[aria-label='Three Canvas']", chromedp.ByQuery)); err != nil {
		return err
	}

	// 3. Take a screenshot for visual confirmation
	shot := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v1", "test", "screenshots", "test_step_4.png")
	_ = os.MkdirAll(filepath.Dir(shot), 0755)
	return session.CaptureScreenshot(shot)
}
