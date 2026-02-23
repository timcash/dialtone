package main

import (
	"time"

	test_v2 "dialtone/dev/plugins/test/src_v1/go"
)

func Run16VideoSectionValidation() error {
	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := navigateToSection(session, "video"); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabel("Test Video")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("Test Video", "data-playing", "true", 4*time.Second)); err != nil {
		return err
	}

	shot, err := screenshotPath("test_step_6.png")
	if err != nil {
		return err
	}
	return session.CaptureScreenshot(shot)
}
