package main

import (
	"os"
	"path/filepath"

	test_v2 "dialtone/dev/plugins/test"
)

func Run12DocsSectionValidation() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := session.Run(test_v2.NavigateToSection("docs", "Docs Section")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabel("Docs Title")); err != nil {
		return err
	}

	shot := filepath.Join(repoRoot, "src", "plugins", "cloudflare", "src_v1", "screenshots", "test_step_2.png")
	return session.CaptureScreenshot(shot)
}
