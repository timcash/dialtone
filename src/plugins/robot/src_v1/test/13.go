package test

import (
	"fmt"
	"time"

	test_v2 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/chromedp/chromedp"
)

func Run13TableSectionValidation(ctx *testCtx) (string, error) {
	session, err := ctx.browser()
	if err != nil {
		return "", err
	}

	fmt.Println("   [STEP] Navigating to Table Section...")
	if err := session.Run(test_v2.NavigateToSection("robot", "table", "Telemetry Section")); err != nil {
		return "", fmt.Errorf("failed navigating to Table: %w", err)
	}

	fmt.Println("   [STEP] Waiting for Robot Table...")
	if err := ctx.waitAria("Robot Table", "table visibility"); err != nil {
		return "", fmt.Errorf("failed waiting for Robot Table: %w", err)
	}

	fmt.Println("   [STEP] Waiting for data-ready=true...")
	if err := ctx.waitAriaAttrEquals("Robot Table", "data-ready", "true", "table ready", 3*time.Second); err != nil {
		return "", fmt.Errorf("failed waiting for data-ready: %w", err)
	}

	fmt.Println("   [STEP] Waiting for table rows...")
	var rowCount int
	start := time.Now()
	for time.Since(start) < 5*time.Second {
		if err := session.Run(chromedp.Evaluate(`document.querySelectorAll("table[aria-label='Robot Table'] tbody tr").length`, &rowCount)); err != nil {
			return "", err
		}
		if rowCount > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if rowCount == 0 {
		return "", fmt.Errorf("robot table has no rows after waiting")
	}

	if err := ctx.captureShot("test_step_3.png"); err != nil {
		return "", err
	}
	return "Table section validated with data rows.", nil
}
