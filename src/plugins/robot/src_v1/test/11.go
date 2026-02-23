package test

import (
	"fmt"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func Run11HeroSectionValidation(ctx *testCtx) (string, error) {
	logs.InfoFromTest("robot-test", "[STEP] Navigating to Hero Section...")
	if err := ctx.navigateSection("hero"); err != nil {
		return "", fmt.Errorf("failed navigating to Hero: %w", err)
	}

	logs.InfoFromTest("robot-test", "[STEP] Waiting for Hero Section...")
	if err := ctx.waitAria("Hero Section", "Hero section visibility"); err != nil {
		return "", fmt.Errorf("failed waiting for Hero Section: %w", err)
	}

	logs.InfoFromTest("robot-test", "[STEP] Waiting for Hero Canvas...")
	if err := ctx.waitAria("Hero Canvas", "Hero canvas visibility"); err != nil {
		return "", fmt.Errorf("failed waiting for Hero Canvas: %w", err)
	}

	if err := ctx.captureShot("test_step_1.png"); err != nil {
		return "", err
	}
	return "Hero section and canvas validated visible.", nil
}
