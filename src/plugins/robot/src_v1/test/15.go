package test

import (
	"fmt"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func Run15XtermSectionValidation(ctx *testCtx) (string, error) {
	logs.InfoFromTest("robot-test", "[STEP] Navigating to Xterm Section...")
	if err := ctx.navigateSection("xterm"); err != nil {
		return "", fmt.Errorf("failed navigating to Xterm: %w", err)
	}

	logs.InfoFromTest("robot-test", "[STEP] Waiting for Xterm Terminal...")
	if err := ctx.waitAria("Xterm Terminal", "terminal visibility"); err != nil {
		return "", fmt.Errorf("failed waiting for Xterm Terminal: %w", err)
	}

	logs.InfoFromTest("robot-test", "[STEP] Waiting for data-ready=true...")
	if err := ctx.waitAriaAttrEquals("Xterm Terminal", "data-ready", "true", "terminal ready", 3*time.Second); err != nil {
		return "", fmt.Errorf("failed waiting for data-ready: %w", err)
	}

	logs.InfoFromTest("robot-test", "[STEP] Waiting for Log Command Input...")
	if err := ctx.waitAria("Log Command Input", "input visibility"); err != nil {
		return "", fmt.Errorf("failed waiting for Log Command Input: %w", err)
	}

	const cmd = "status --verbose"
	if err := ctx.typeAndSubmitAria("Log Command Input", cmd, "typing command"); err != nil {
		return "", fmt.Errorf("failed typing command: %w", err)
	}

	logs.InfoFromTest("robot-test", "[STEP] Waiting for command echo...")
	if err := ctx.waitAriaAttrEquals("Xterm Terminal", "data-last-command", cmd, "command echo check", 3*time.Second); err != nil {
		return "", fmt.Errorf("failed waiting for command echo: %w", err)
	}

	if err := ctx.captureShot("test_step_5.png"); err != nil {
		return "", err
	}
	return "Xterm section validated with command execution.", nil
}
