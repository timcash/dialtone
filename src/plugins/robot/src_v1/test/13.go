package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	test_v2 "dialtone/cli/src/libs/test_v2"
	"github.com/chromedp/chromedp"
)

func Run13TableSectionValidation() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	fmt.Println("   [STEP] Navigating to Table Section...")
	if err := session.Run(test_v2.NavigateToSection("table", "Telemetry Section")); err != nil {
		return fmt.Errorf("failed navigating to Table: %w", err)
	}

	fmt.Println("   [STEP] Waiting for Robot Table...")
	if err := session.Run(test_v2.WaitForAriaLabel("Robot Table")); err != nil {
		return fmt.Errorf("failed waiting for Robot Table: %w", err)
	}

	fmt.Println("   [STEP] Waiting for data-ready=true...")
	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("Robot Table", "data-ready", "true", 3*time.Second)); err != nil {
		return fmt.Errorf("failed waiting for data-ready: %w", err)
	}

	fmt.Println("   [STEP] Waiting for table rows...")
	var rowCount int
	start := time.Now()
	for time.Since(start) < 5*time.Second {
		if err := session.Run(chromedp.Evaluate(`document.querySelectorAll("table[aria-label='Robot Table'] tbody tr").length`, &rowCount)); err != nil {
			return err
		}
		if rowCount > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if rowCount == 0 {
		return fmt.Errorf("robot table has no rows after waiting")
	}

	shot := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v1", "screenshots", "test_step_3.png")
	return session.CaptureScreenshot(shot)
}
