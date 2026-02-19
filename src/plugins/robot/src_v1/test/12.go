package main

import (
	"fmt"

	test_v2 "dialtone/dev/libs/test_v2"
)

func Run12DocsSectionValidation(ctx *testCtx) (string, error) {
	session, err := ctx.browser()
	if err != nil {
		return "", err
	}

	fmt.Println("   [STEP] Navigating to Docs Section...")
	if err := session.Run(test_v2.NavigateToSection("docs", "Docs Section")); err != nil {
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
