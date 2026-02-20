package main

import (
	"os"
	"path/filepath"
	"time"

	test_v2 "dialtone/dev/plugins/test/src_v1/go"
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

	if err := session.Run(test_v2.NavigateToSection("table", "Table Section")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabel("VPN Table")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("VPN Table", "data-ready", "true", 3*time.Second)); err != nil {
		return err
	}

	shot := filepath.Join(repoRoot, "src", "plugins", "vpn", "src_v1", "screenshots", "test_step_3.png")
	return session.CaptureScreenshot(shot)
}
