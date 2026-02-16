package main

import (
	"os"
	"path/filepath"
	"time"

	test_v2 "dialtone/cli/src/libs/test_v2"
)

func Run15XtermSectionValidation() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := session.Run(test_v2.NavigateToSection("xterm", "Xterm Section")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabel("Xterm Terminal")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-ready", "true", 3*time.Second)); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabel("Xterm Input")); err != nil {
		return err
	}
	const cmd = "status --verbose"
	if err := session.Run(test_v2.TypeAndSubmitAriaLabel("Xterm Input", cmd)); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-last-command", cmd, 3*time.Second)); err != nil {
		return err
	}

	shot := filepath.Join(repoRoot, "src", "plugins", "vpn", "src_v1", "screenshots", "test_step_5.png")
	return session.CaptureScreenshot(shot)
}
