package main

import (
	"time"

	test_v2 "dialtone/dev/plugins/test/src_v1/go"
)

func Run13TableSectionValidation() error {
	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := navigateToSection(session, "status"); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabel("Tunnel Table")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("Tunnel Table", "data-ready", "true", 3*time.Second)); err != nil {
		return err
	}

	shot, err := screenshotPath("test_step_3.png")
	if err != nil {
		return err
	}
	return session.CaptureScreenshot(shot)
}
