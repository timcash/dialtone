package test

import (
	"fmt"
)

func Run12DocsSectionValidation(ctx *testCtx) (string, error) {
	fmt.Println("   [STEP] Navigating to Docs Section...")
	if err := ctx.navigateSection("docs"); err != nil {
		return "", fmt.Errorf("failed navigating to Docs: %w", err)
	}

	fmt.Println("   [STEP] Waiting for Docs Content...")
	if err := ctx.waitAria("Docs Content", "Docs content visibility"); err != nil {
		return "", fmt.Errorf("failed waiting for Docs Content: %w", err)
	}

	if err := ctx.captureShot("test_step_2.png"); err != nil {
		return "", err
	}
	return "Docs section navigation and content validated.", nil
}
