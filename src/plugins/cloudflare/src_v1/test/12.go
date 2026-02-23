package main

import (
	test_v2 "dialtone/dev/plugins/test/src_v1/go"
)

func Run12DocsSectionValidation() error {
	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := navigateToSection(session, "docs"); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabel("Docs Title")); err != nil {
		return err
	}

	shot, err := screenshotPath("test_step_2.png")
	if err != nil {
		return err
	}
	return session.CaptureScreenshot(shot)
}
