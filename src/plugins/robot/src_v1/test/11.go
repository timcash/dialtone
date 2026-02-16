package main

import (
	"os"
	"path/filepath"

	"github.com/chromedp/chromedp"
)

func Run11HeroSectionValidation() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := session.Run(chromedp.Tasks{
		chromedp.WaitVisible("[aria-label='Hero Section']", chromedp.ByQuery),
		chromedp.WaitVisible("[aria-label='Hero Canvas']", chromedp.ByQuery),
	}); err != nil {
		return err
	}

	shot := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v1", "screenshots", "test_step_1.png")
	return session.CaptureScreenshot(shot)
}
