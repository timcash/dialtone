package main

import (
	"fmt"
	"time"

	test_v2 "dialtone/dev/libs/test_v2"
)

func Run16VideoSectionValidation(ctx *testCtx) (string, error) {
	session, err := ctx.browser()
	if err != nil {
		return "", err
	}

	fmt.Println("   [STEP] Navigating to Video Section...")
	if err := session.Run(test_v2.NavigateToSection("video", "Video Section")); err != nil {
		return "", fmt.Errorf("failed navigating to Video: %w", err)
	}

	fmt.Println("   [STEP] Waiting for video playback (data-playing=true)...")
	if err := ctx.waitAriaAttrEquals("Video Section", "data-playing", "true", "video playing", 4*time.Second); err != nil {
		return "", fmt.Errorf("failed waiting for video playback: %w", err)
	}

	if err := ctx.captureShot("test_step_6.png"); err != nil {
		return "", err
	}
	return "Video section validated with playback.", nil
}
