package main

import (
	"fmt"
	"time"

	test_v2 "dialtone/dev/plugins/test"
)

func Run15XtermSectionValidation(ctx *testCtx) (string, error) {
	session, err := ctx.browser()
	if err != nil {
		return "", err
	}

	fmt.Println("   [STEP] Navigating to Xterm Section...")
	if err := session.Run(test_v2.NavigateToSection("xterm", "Xterm Section")); err != nil {
		return "", fmt.Errorf("failed navigating to Xterm: %w", err)
	}

	fmt.Println("   [STEP] Waiting for Xterm Terminal...")
	if err := ctx.waitAria("Xterm Terminal", "terminal visibility"); err != nil {
		return "", fmt.Errorf("failed waiting for Xterm Terminal: %w", err)
	}

	fmt.Println("   [STEP] Waiting for data-ready=true...")
	if err := ctx.waitAriaAttrEquals("Xterm Terminal", "data-ready", "true", "terminal ready", 3*time.Second); err != nil {
		return "", fmt.Errorf("failed waiting for data-ready: %w", err)
	}

	fmt.Println("   [STEP] Waiting for Xterm Input...")
	if err := ctx.waitAria("Xterm Input", "input visibility"); err != nil {
		return "", fmt.Errorf("failed waiting for Xterm Input: %w", err)
	}

	const cmd = "status --verbose"
	if err := ctx.typeAndSubmitAria("Xterm Input", cmd, "typing command"); err != nil {
		return "", fmt.Errorf("failed typing command: %w", err)
	}

	fmt.Println("   [STEP] Waiting for command echo...")
	if err := ctx.waitAriaAttrEquals("Xterm Terminal", "data-last-command", cmd, "command echo check", 3*time.Second); err != nil {
		return "", fmt.Errorf("failed waiting for command echo: %w", err)
	}

	if err := ctx.captureShot("test_step_5.png"); err != nil {
		return "", err
	}
	return "Xterm section validated with command execution.", nil
}
