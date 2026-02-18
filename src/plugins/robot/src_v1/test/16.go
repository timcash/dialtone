package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	test_v2 "dialtone/cli/src/libs/test_v2"
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

	fmt.Println("   [STEP] Navigating to Video Section...")
	if err := session.Run(test_v2.NavigateToSection("video", "Video Section")); err != nil {
		return fmt.Errorf("failed navigating to Video: %w", err)
	}

	fmt.Println("   [STEP] Waiting for video playback (data-playing=true)...")
	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("Video Section", "data-playing", "true", 4*time.Second)); err != nil {
		return fmt.Errorf("failed waiting for video playback: %w", err)
	}

	shot := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v1", "screenshots", "test_step_6.png")
	return session.CaptureScreenshot(shot)
}
