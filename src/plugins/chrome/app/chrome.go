package chrome

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"dialtone/cli/src/core/browser"
	"dialtone/cli/src/core/logger"
	"github.com/chromedp/chromedp"
)

// VerifyChrome attempts to find and connect to a Chrome/Chromium browser.
func VerifyChrome(port int, debug bool) error {
	path := browser.FindChromePath()
	if path == "" {
		return fmt.Errorf("no Chrome or Chromium browser found in PATH or standard locations for %s", runtime.GOOS)
	}

	if debug {
		logger.LogInfo("DEBUG: System OS: %s", runtime.GOOS)
		logger.LogInfo("DEBUG: Selected Browser Path: %s", path)
	}

	// Automated Cleanup: Kill any process on the target port to avoid connection refusal
	if err := browser.CleanupPort(port); err != nil {
		logger.LogInfo("Warning: Failed to cleanup port %d: %v", port, err)
	}

	logger.LogInfo("Browser check: Found %s", path)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.ExecPath(path),
		chromedp.Flag("remote-debugging-port", fmt.Sprintf("%d", port)),
		chromedp.Flag("remote-debugging-address", "127.0.0.1"), // Force IPv4 to avoid [::1] connection issues on WSL
		chromedp.Flag("disable-gpu", true),
	)

	if debug {
		logger.LogInfo("DEBUG: Initializing allocator on port %d...", port)
		// Set output to see browser logs in debug mode
		opts = append(opts, chromedp.CombinedOutput(os.Stderr))
	}

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer func() {
		if debug {
			logger.LogInfo("DEBUG: Shutting down allocator...")
		}
		cancel()
	}()

	logger.LogInfo("Chrome: Starting browser instance...")
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer func() {
		logger.LogInfo("Chrome: Stopping browser...")
		cancel()
	}()

	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	logger.LogInfo("Chrome: Navigating to about:blank...")
	var title string
	err := chromedp.Run(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.Title(&title),
	)

	if err != nil {
		return fmt.Errorf("failed to run chromedp: %w", err)
	}

	logger.LogInfo("Chrome: Page loaded successfully. Title: '%s'", title)
	return nil
}

