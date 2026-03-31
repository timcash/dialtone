package main

import (
	"time"
)

func Run16VideoSectionValidation() error {
	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := navigateToSection(session, "video"); err != nil {
		return err
	}
	if err := session.WaitForAriaLabel("Test Video", 5*time.Second); err != nil {
		return err
	}
	if err := session.WaitForAriaLabelAttrEquals("Test Video", "data-playing", "true", 4*time.Second); err != nil {
		return err
	}

	shot, err := screenshotPath("test_step_6.png")
	if err != nil {
		return err
	}
	return session.CaptureScreenshot(shot)
}
