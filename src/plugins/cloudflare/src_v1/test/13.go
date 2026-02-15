package main

import (
	"os"
	"path/filepath"
	"time"

	test_v2 "dialtone/cli/src/libs/test_v2"
)

func Run13TableSectionValidation() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := session.Run(test_v2.NavigateToSection("status", "Status Section")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabel("Tunnel Table")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("Tunnel Table", "data-ready", "true", 3*time.Second)); err != nil {
		return err
	}

	shot := filepath.Join(repoRoot, "src", "plugins", "cloudflare", "src_v1", "screenshots", "test_step_3.png")
	return session.CaptureScreenshot(shot)
}
