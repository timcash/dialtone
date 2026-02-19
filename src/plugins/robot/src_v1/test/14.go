package main

import (
	"fmt"

	test_v2 "dialtone/dev/libs/test_v2"
)

func Run14ThreeSectionValidation(ctx *testCtx) (string, error) {
	session, err := ctx.browser()
	if err != nil {
		return "", err
	}

	// 1. Navigate to 3D Section
	fmt.Println("   [STEP] Navigating to Three Section...")
	if err := session.Run(test_v2.NavigateToSection("three", "Three Section")); err != nil {
		return "", fmt.Errorf("failed navigating to Three: %w", err)
	}

	// 2. Verify Canvas is visible
	fmt.Println("   [STEP] Waiting for Three Canvas...")
	if err := ctx.waitAria("Three Canvas", "Three canvas visibility"); err != nil {
		return "", fmt.Errorf("failed waiting for Three Canvas: %w", err)
	}

	// 3. Take a screenshot for visual confirmation
	if err := ctx.captureShot("test_step_4.png"); err != nil {
		return "", err
	}
	return "Three section navigation and canvas validated.", nil
}
