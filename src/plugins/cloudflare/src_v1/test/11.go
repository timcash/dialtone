package main

import "time"

func Run11HeroSectionValidation() error {
	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := session.WaitForAriaLabel("Hero Section", 5*time.Second); err != nil {
		return err
	}
	if err := session.WaitForAriaLabel("Hero Canvas", 5*time.Second); err != nil {
		return err
	}

	shot, err := screenshotPath("test_step_1.png")
	if err != nil {
		return err
	}
	return session.CaptureScreenshot(shot)
}
