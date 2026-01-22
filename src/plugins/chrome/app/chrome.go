package chrome

import (
	"context"
	"fmt"
	"os"
	"time"

	"dialtone/cli/src/core/logger"
	"github.com/chromedp/chromedp"
)

// VerifyChrome attempts to find and connect to a Chrome/Chromium browser.
func VerifyChrome() error {
	path := FindChromePath()
	if path == "" {
		return fmt.Errorf("no Chrome or Chromium browser found in PATH or standard locations")
	}

	logger.LogInfo("Found browser at: %s", path)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.ExecPath(path),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var title string
	err := chromedp.Run(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.Title(&title),
	)

	if err != nil {
		return fmt.Errorf("failed to run chromedp: %w", err)
	}

	logger.LogInfo("Successfully connected to browser. Title: %s", title)
	return nil
}

// FindChromePath looks for Chrome/Chromium in common locations.
func FindChromePath() string {
	// Common Linux paths
	paths := []string{
		"/usr/bin/google-chrome",
		"/usr/bin/chromium-browser",
		"/usr/bin/chromium",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// WSL Paths to Windows Chrome
	wslPaths := []string{
		"/mnt/c/Program Files/Google/Chrome/Application/chrome.exe",
		"/mnt/c/Program Files (x86)/Google/Chrome/Application/chrome.exe",
	}

	for _, p := range wslPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}
