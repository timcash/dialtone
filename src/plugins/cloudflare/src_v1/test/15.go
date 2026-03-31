package main

import (
	"time"
)

func Run15XtermSectionValidation() error {
	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := navigateToSection(session, "xterm"); err != nil {
		return err
	}
	if err := session.WaitForAriaLabel("Xterm Terminal", 5*time.Second); err != nil {
		return err
	}
	if err := session.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-ready", "true", 3*time.Second); err != nil {
		return err
	}
	if err := session.WaitForAriaLabel("Xterm Input", 5*time.Second); err != nil {
		return err
	}
	const cmd = "status --verbose"
	if err := session.TypeAriaLabel("Xterm Input", cmd); err != nil {
		return err
	}
	if err := session.PressEnterAriaLabel("Xterm Input"); err != nil {
		return err
	}
	if err := session.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-last-command", cmd, 3*time.Second); err != nil {
		return err
	}

	shot, err := screenshotPath("test_step_5.png")
	if err != nil {
		return err
	}
	return session.CaptureScreenshot(shot)
}
