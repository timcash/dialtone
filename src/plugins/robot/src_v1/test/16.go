package test

import (
	"fmt"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func Run16VideoSectionValidation(ctx *testCtx) (string, error) {
	logs.InfoFromTest("robot-test", "[STEP] Navigating to Video Section...")
	if err := ctx.navigateSection("video"); err != nil {
		return "", fmt.Errorf("failed navigating to Video: %w", err)
	}

	logs.InfoFromTest("robot-test", "[STEP] Waiting for video playback (data-playing=true)...")
	if err := ctx.waitAriaAttrEquals("Video Section", "data-playing", "true", "video playing", 4*time.Second); err != nil {
		return "", fmt.Errorf("failed waiting for video playback: %w", err)
	}

	if err := ctx.captureShot("test_step_6.png"); err != nil {
		return "", err
	}
	return "Video section validated with playback.", nil
}
