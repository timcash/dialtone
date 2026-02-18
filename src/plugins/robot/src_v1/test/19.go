package main

import (
	"fmt"

	"github.com/chromedp/chromedp"
)

func Run19MenuNavigationValidation(ctx *testCtx) (string, error) {
	session, err := ctx.browser()
	if err != nil {
		return "", err
	}

	// 1. Initial State (Hero)
	if err := ctx.captureShot("menu_1_hero.png"); err != nil {
		return "", fmt.Errorf("failed to capture hero screenshot: %w", err)
	}

	// 2. Click the menu toggle button
	if err := ctx.clickAria("Toggle Global Menu", "open menu"); err != nil {
		return "", fmt.Errorf("failed to click menu toggle: %w", err)
	}
	if err := ctx.waitAria("Global Menu Panel", "menu visible"); err != nil {
		return "", fmt.Errorf("menu panel not visible: %w", err)
	}

	if err := ctx.captureShot("menu_2_open.png"); err != nil {
		return "", fmt.Errorf("failed to capture menu open screenshot: %w", err)
	}

	// 3. Click a navigation item (e.g., "Telemetry")
	if err := ctx.clickAria("Navigate Telemetry", "nav to telemetry"); err != nil {
		return "", fmt.Errorf("failed to navigate to Telemetry: %w", err)
	}

	fmt.Println("   [STEP] Waiting for menu to close and Telemetry to activate...")
	if err := session.Run(chromedp.WaitNotVisible("[aria-label='Global Menu Panel']", chromedp.ByQuery)); err != nil {
		return "", fmt.Errorf("menu did not close: %w", err)
	}
	if err := session.Run(chromedp.WaitVisible("[aria-label='Telemetry Section'][data-active='true']", chromedp.ByQuery)); err != nil {
		return "", fmt.Errorf("telemetry section not active: %w", err)
	}

	if err := ctx.captureShot("menu_3_telemetry.png"); err != nil {
		return "", fmt.Errorf("failed to capture telemetry section screenshot: %w", err)
	}

	return "Menu navigation flow validated.", nil
}
