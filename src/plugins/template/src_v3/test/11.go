package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"dialtone/cli/src/core/browser"
	test_v2 "dialtone/cli/src/libs/test_v2"
	"github.com/chromedp/chromedp"
)

func Run11HeroSectionValidation() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	_ = browser.CleanupPort(8080)

	serve := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "template", "serve", "src_v3")
	serve.Dir = repoRoot
	serve.Stdout = os.Stdout
	serve.Stderr = os.Stderr
	if err := serve.Start(); err != nil {
		return err
	}
	defer func() {
		_ = serve.Process.Kill()
		_, _ = serve.Process.Wait()
	}()

	if err := waitForPort("127.0.0.1:8080", 12*time.Second); err != nil {
		return err
	}

	session, err := test_v2.StartBrowser(test_v2.BrowserOptions{
		Headless:      true,
		Role:          "test",
		ReuseExisting: false,
		URL:           "http://127.0.0.1:8080",
		LogWriter:     os.Stdout,
		LogPrefix:     "[BROWSER]",
	})
	if err != nil {
		return err
	}
	defer session.Close()

	if err := session.Run(chromedp.Tasks{
		chromedp.WaitVisible("[aria-label='Hero Section']", chromedp.ByQuery),
		chromedp.WaitVisible("[aria-label='Hero Canvas']", chromedp.ByQuery),
	}); err != nil {
		return err
	}

	shot := filepath.Join(repoRoot, "src", "plugins", "template", "src_v3", "screenshots", "test_step_1.png")
	return session.CaptureScreenshot(shot)
}
