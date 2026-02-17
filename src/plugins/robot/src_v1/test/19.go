package main

import (
	"fmt"
	"github.com/chromedp/chromedp"
)

func Run19MenuNavigationValidation() error {
	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	// 1. Initial State (Hero)
	if err := captureStoryShot(session, "menu_1_hero.png"); err != nil {
		return fmt.Errorf("failed to capture hero screenshot: %w", err)
	}

	// 2. Click the menu toggle button
	if err := session.Run(chromedp.Tasks{
		chromedp.Click("[aria-label='Toggle Global Menu']", chromedp.ByQuery),
		chromedp.WaitVisible("[aria-label='Global Menu Panel']", chromedp.ByQuery),
	}); err != nil {
		return fmt.Errorf("failed to click menu toggle or menu panel not visible: %w", err)
	}

	if err := captureStoryShot(session, "menu_2_open.png"); err != nil {
		return fmt.Errorf("failed to capture menu open screenshot: %w", err)
	}

	// 3. Click a navigation item (e.g., "Telemetry")
	if err := session.Run(chromedp.Tasks{
		chromedp.Click("[aria-label='Navigate Telemetry']", chromedp.ByQuery),
		chromedp.WaitNotVisible("[aria-label='Global Menu Panel']", chromedp.ByQuery), // Menu should close
		chromedp.WaitVisible("[aria-label='Telemetry Section'][data-active='true']", chromedp.ByQuery),
	}); err != nil {
		return fmt.Errorf("failed to navigate to Telemetry section: %w", err)
	}

	if err := captureStoryShot(session, "menu_3_telemetry.png"); err != nil {
		return fmt.Errorf("failed to capture telemetry section screenshot: %w", err)
	}

	return nil
}
