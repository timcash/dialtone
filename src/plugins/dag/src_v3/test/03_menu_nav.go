package main

import (
	"fmt"
	"time"

	test_v2 "dialtone/cli/src/libs/test_v2"
	"github.com/chromedp/chromedp"
)

func Run03MenuNavSectionSwitch() error {
	browser, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := browser.Run(chromedp.Tasks{
		chromedp.Navigate("http://127.0.0.1:8080/#dag-table"),
		test_v2.WaitForAriaLabel("Toggle Global Menu"),
		test_v2.WaitForAriaLabel("DAG Table"),
		test_v2.WaitForAriaLabelAttrEquals("DAG Table", "data-ready", "true", 8*time.Second),
		test_v2.ClickAriaLabel("Toggle Global Menu"),
		test_v2.WaitForAriaLabel("Navigate Stage"),
	}); err != nil {
		return fmt.Errorf("menu nav section switch failed: %w", err)
	}
	if err := captureStoryShot(browser, "test_step_menu_nav_pre.png"); err != nil {
		return fmt.Errorf("capture menu nav pre screenshot: %w", err)
	}
	if err := browser.Run(chromedp.Tasks{
		test_v2.ClickAriaLabel("Navigate Stage"),
		test_v2.WaitForAriaLabel("Three Canvas"),
		test_v2.WaitForAriaLabelAttrEquals("Three Canvas", "data-ready", "true", 6*time.Second),
	}); err != nil {
		return fmt.Errorf("menu nav stage transition failed: %w", err)
	}
	if err := captureStoryShot(browser, "test_step_menu_nav.png"); err != nil {
		return fmt.Errorf("capture menu nav screenshot: %w", err)
	}

	return nil
}
