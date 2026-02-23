package test

import (
	"fmt"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func Run14ThreeSectionValidation(ctx *testCtx) (string, error) {
	// 1. Navigate to 3D Section
	logs.InfoFromTest("robot-test", "[STEP] Navigating to Three Section...")
	if err := ctx.navigateSection("three"); err != nil {
		return "", fmt.Errorf("failed navigating to Three: %w", err)
	}

	// 2. Verify Canvas is visible
	logs.InfoFromTest("robot-test", "[STEP] Waiting for Three Canvas...")
	if err := ctx.waitAria("Three Canvas", "Three canvas visibility"); err != nil {
		return "", fmt.Errorf("failed waiting for Three Canvas: %w", err)
	}

	// 3. Take a screenshot for visual confirmation
	if err := ctx.captureShot("test_step_4.png"); err != nil {
		return "", err
	}
	return "Three section navigation and canvas validated.", nil
}
