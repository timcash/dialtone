package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	test_v2 "dialtone/cli/src/libs/test_v2"
)

func Run15XtermSectionValidation() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	fmt.Println("   [STEP] Navigating to Xterm Section...")
	if err := session.Run(test_v2.NavigateToSection("xterm", "Xterm Section")); err != nil {
		return fmt.Errorf("failed navigating to Xterm: %w", err)
	}

	fmt.Println("   [STEP] Waiting for Xterm Terminal...")
	if err := session.Run(test_v2.WaitForAriaLabel("Xterm Terminal")); err != nil {
		return fmt.Errorf("failed waiting for Xterm Terminal: %w", err)
	}

	fmt.Println("   [STEP] Waiting for data-ready=true...")
	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-ready", "true", 3*time.Second)); err != nil {
		return fmt.Errorf("failed waiting for data-ready: %w", err)
	}

	fmt.Println("   [STEP] Waiting for Xterm Input...")
	if err := session.Run(test_v2.WaitForAriaLabel("Xterm Input")); err != nil {
		return fmt.Errorf("failed waiting for Xterm Input: %w", err)
	}

	const cmd = "status --verbose"
	fmt.Printf("   [STEP] Typing command: %s...\n", cmd)
	if err := session.Run(test_v2.TypeAndSubmitAriaLabel("Xterm Input", cmd)); err != nil {
		return fmt.Errorf("failed typing command: %w", err)
	}

	fmt.Println("   [STEP] Waiting for command echo...")
	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-last-command", cmd, 3*time.Second)); err != nil {
		return fmt.Errorf("failed waiting for command echo: %w", err)
	}

	shot := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v1", "screenshots", "test_step_5.png")
	return session.CaptureScreenshot(shot)
}
