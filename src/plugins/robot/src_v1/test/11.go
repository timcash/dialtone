package test

import (
	"fmt"
)

func Run11HeroSectionValidation(ctx *testCtx) (string, error) {
	fmt.Println("   [STEP] Waiting for Hero Section...")
	if err := ctx.waitAria("Hero Section", "Hero section visibility"); err != nil {
		return "", fmt.Errorf("failed waiting for Hero Section: %w", err)
	}

	fmt.Println("   [STEP] Waiting for Hero Canvas...")
	if err := ctx.waitAria("Hero Canvas", "Hero canvas visibility"); err != nil {
		return "", fmt.Errorf("failed waiting for Hero Canvas: %w", err)
	}

	if err := ctx.captureShot("test_step_1.png"); err != nil {
		return "", err
	}
	return "Hero section and canvas validated visible.", nil
}
