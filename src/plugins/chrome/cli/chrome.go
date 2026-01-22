package cli

import (
	"flag"
	"fmt"

	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/plugins/chrome/app"
)

// RunChrome handles the 'chrome' command
func RunChrome(args []string) {
	fs := flag.NewFlagSet("chrome", flag.ExitOnError)
	help := fs.Bool("help", false, "Show help for chrome command")

	fs.Parse(args)

	if *help {
		printChromeUsage()
		return
	}

	logger.LogInfo("Verifying Chrome/Chromium connectivity...")
	if err := chrome.VerifyChrome(); err != nil {
		logger.LogFatal("Chrome verification FAILED: %v", err)
	}

	logger.LogInfo("Chrome verification SUCCESS")
}

func printChromeUsage() {
	fmt.Println("Usage: dialtone chrome")
	fmt.Println()
	fmt.Println("Verify Chrome/Chromium connectivity via chromedp.")
	fmt.Println("Detects local installations on Linux, macOS, and WSL (Windows Chrome).")
}
