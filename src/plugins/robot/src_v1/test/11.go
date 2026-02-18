package main

import (
	"fmt"
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

	fmt.Println("   [STEP] Waiting for Hero Section...")
	if err := session.Run(chromedp.WaitVisible("[aria-label='Hero Section']", chromedp.ByQuery)); err != nil {
		return fmt.Errorf("failed waiting for Hero Section: %w", err)
	}

	fmt.Println("   [STEP] Waiting for Hero Canvas...")
	if err := session.Run(chromedp.WaitVisible("[aria-label='Hero Canvas']", chromedp.ByQuery)); err != nil {
		return fmt.Errorf("failed waiting for Hero Canvas: %w", err)
	}

	shot := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v1", "screenshots", "test_step_1.png")
	return session.CaptureScreenshot(shot)
}
