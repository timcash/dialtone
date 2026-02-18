package main

import (
	"fmt"
	"os"
	"path/filepath"

	test_v2 "dialtone/cli/src/libs/test_v2"
)

func Run12DocsSectionValidation() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	fmt.Println("   [STEP] Navigating to Docs Section...")
	if err := session.Run(test_v2.NavigateToSection("docs", "Docs Section")); err != nil {
		return fmt.Errorf("failed navigating to Docs: %w", err)
	}

	fmt.Println("   [STEP] Waiting for Docs Content...")
	if err := session.Run(test_v2.WaitForAriaLabel("Docs Content")); err != nil {
		return fmt.Errorf("failed waiting for Docs Content: %w", err)
	}

	shot := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v1", "screenshots", "test_step_2.png")
	return session.CaptureScreenshot(shot)
}
