package main

import "time"

func Run13TableSectionValidation() error {
	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := navigateToSection(session, "status"); err != nil {
		return err
	}
	if err := session.WaitForAriaLabel("Tunnel Table", 5*time.Second); err != nil {
		return err
	}
	if err := session.WaitForAriaLabelAttrEquals("Tunnel Table", "data-ready", "true", 3*time.Second); err != nil {
		return err
	}

	shot, err := screenshotPath("test_step_3.png")
	if err != nil {
		return err
	}
	return session.CaptureScreenshot(shot)
}
