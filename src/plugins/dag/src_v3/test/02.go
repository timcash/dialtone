package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	test_v2 "dialtone/cli/src/libs/test_v2"
	"github.com/chromedp/chromedp"
)

func Run02DagTableSectionValidation() error {
	browser, err := ensureSharedBrowser(true)
	if err != nil {
		return err
	}

	var tableOK bool
	if err := browser.Run(chromedp.Tasks{
		chromedp.Navigate("http://127.0.0.1:8080/#dag-table"),
		test_v2.WaitForAriaLabel("DAG Table"),
		test_v2.WaitForAriaLabelAttrEquals("DAG Table", "data-ready", "true", 8*time.Second),
		chromedp.Evaluate(`
			(() => {
				const table = document.querySelector("table[aria-label='DAG Table']");
				if (!table) return false;
				const rows = table.querySelectorAll('tbody tr');
				if (rows.length < 5) return false;
				const first = rows[0].querySelector('td');
				if (!first) return false;
				return first.textContent?.trim() === 'node_count';
			})()
		`, &tableOK),
	}); err != nil {
		return err
	}
	if !tableOK {
		return fmt.Errorf("dag-table assertions failed")
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	shot := filepath.Join(repoRoot, "src", "plugins", "dag", "src_v3", "screenshots", "test_step_1.png")
	if err := browser.CaptureScreenshot(shot); err != nil {
		return fmt.Errorf("capture table screenshot: %w", err)
	}
	return nil
}
