package test

import (
	"fmt"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func Run12DocsSectionValidation(ctx *testCtx) (string, error) {
	logs.InfoFromTest("robot-test", "[STEP] Navigating to Docs Section...")
	if err := ctx.navigateSection("docs"); err != nil {
		return "", fmt.Errorf("failed navigating to Docs: %w", err)
	}

	logs.InfoFromTest("robot-test", "[STEP] Waiting for Docs Content...")
	if err := ctx.waitAria("Docs Content", "Docs content visibility"); err != nil {
		return "", fmt.Errorf("failed waiting for Docs Content: %w", err)
	}

	if err := ctx.captureShot("test_step_2.png"); err != nil {
		return "", err
	}
	return "Docs section navigation and content validated.", nil
}
