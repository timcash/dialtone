package main

import (
	"os"
	"path/filepath"
	"time"

	test_v2 "dialtone/dev/plugins/dag/src_v3/suite"
)

func Run16VideoSectionValidation() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := session.Run(test_v2.NavigateToSection("video", "Video Section")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabel("Test Video")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("Test Video", "data-playing", "true", 4*time.Second)); err != nil {
		return err
	}

	shot := filepath.Join(repoRoot, "src", "plugins", "cloudflare", "src_v1", "screenshots", "test_step_6.png")
	return session.CaptureScreenshot(shot)
}
