package main

import "time"

func Run12DocsSectionValidation() error {
	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := navigateToSection(session, "docs"); err != nil {
		return err
	}
	if err := session.WaitForAriaLabel("Docs Title", 5*time.Second); err != nil {
		return err
	}

	shot, err := screenshotPath("test_step_2.png")
	if err != nil {
		return err
	}
	return session.CaptureScreenshot(shot)
}
