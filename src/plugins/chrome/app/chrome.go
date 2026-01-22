package chrome

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"dialtone/cli/src/core/logger"
	"github.com/chromedp/chromedp"
)

// VerifyChrome attempts to find and connect to a Chrome/Chromium browser.
func VerifyChrome(port int, debug bool) error {
	path := FindChromePath()
	if path == "" {
		return fmt.Errorf("no Chrome or Chromium browser found in PATH or standard locations for %s", runtime.GOOS)
	}

	if debug {
		logger.LogInfo("DEBUG: System OS: %s", runtime.GOOS)
		logger.LogInfo("DEBUG: Selected Browser Path: %s", path)
	}

	logger.LogInfo("Browser check: Found %s", path)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.ExecPath(path),
		chromedp.Flag("remote-debugging-port", fmt.Sprintf("%d", port)),
		chromedp.Flag("disable-gpu", true),
	)

	if debug {
		logger.LogInfo("DEBUG: Initializing allocator on port %d...", port)
		logger.LogInfo("DEBUG: Flags: %v", opts)
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

// FindChromePath looks for Chrome/Chromium in common locations based on the OS.
func FindChromePath() string {
	switch runtime.GOOS {
	case "darwin":
		paths := []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			os.Getenv("HOME") + "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	case "windows":
		// Check both Program Files and Local AppData (user install)
		programFiles := os.Getenv("ProgramFiles")
		if programFiles == "" {
			programFiles = `C:\Program Files`
		}
		programFilesX86 := os.Getenv("ProgramFiles(x86)")
		if programFilesX86 == "" {
			programFilesX86 = `C:\Program Files (x86)`
		}
		localAppData := os.Getenv("LocalAppData")

		paths := []string{
			filepath.Join(programFiles, `Google\Chrome\Application\chrome.exe`),
			filepath.Join(programFilesX86, `Google\Chrome\Application\chrome.exe`),
		}
		if localAppData != "" {
			paths = append(paths, filepath.Join(localAppData, `Google\Chrome\Application\chrome.exe`))
		}

		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	case "linux":
		// Standard Linux paths (stable, beta, unstable)
		paths := []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/google-chrome-beta",
			"/usr/bin/google-chrome-unstable",
			"/usr/bin/chromium-browser",
			"/usr/bin/chromium",
			"/usr/bin/brave-browser", // Often compatible
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}

		// Fallback for WSL (Windows Host)
		wslPaths := []string{
			"/mnt/c/Program Files/Google/Chrome/Application/chrome.exe",
			"/mnt/c/Program Files (x86)/Google/Chrome/Application/chrome.exe",
		}
		for _, p := range wslPaths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	return ""
}
