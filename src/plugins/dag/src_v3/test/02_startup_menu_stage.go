package main

import (
	"fmt"
	"time"
)

func Run02StartupMenuToStageFresh(ctx *testCtx) (string, error) {
	if _, err := ctx.browser(); err != nil {
		return "", err
	}

	ctx.appendThought("startup nav: reset startup view state and open app fresh")
	if err := ctx.runEval(`(() => {
		try {
			window.sessionStorage.removeItem('dag.src_v3.active_section');
			window.sessionStorage.removeItem('dag.src_v3.api_ready');
		} catch {}
		return true;
	})()`, nil); err != nil {
		return "", fmt.Errorf("clear startup session state: %w", err)
	}

	if err := ctx.navigate("http://127.0.0.1:8080/"); err != nil {
		return "", fmt.Errorf("fresh app navigate failed: %w", err)
	}

	if err := ctx.waitAria("Toggle Global Menu", "fresh startup needs menu toggle"); err != nil {
		return "", err
	}
	if err := ctx.captureShot("test_step_startup_menu_stage_pre.png"); err != nil {
		return "", fmt.Errorf("capture startup pre screenshot: %w", err)
	}
	if err := ctx.clickAria("Toggle Global Menu", "open global menu from fresh startup"); err != nil {
		return "", err
	}
	if err := ctx.waitAria("Navigate Stage", "fresh startup needs stage nav button"); err != nil {
		return "", err
	}
	if err := ctx.clickAria("Navigate Stage", "switch to stage from menu"); err != nil {
		return "", err
	}
	if err := ctx.waitAria("Three Canvas", "stage canvas should exist after menu nav"); err != nil {
		return "", err
	}
	if err := ctx.waitAriaAttrEquals("Three Section", "data-active", "true", "stage section should be active", 6*time.Second); err != nil {
		return "", err
	}
	if err := ctx.waitAriaAttrEquals("Three Canvas", "data-ready", "true", "stage should report ready", 8*time.Second); err != nil {
		return "", err
	}
	if err := ctx.captureShot("test_step_startup_menu_stage.png"); err != nil {
		return "", fmt.Errorf("capture startup stage screenshot: %w", err)
	}

	return "Fresh app startup opened menu immediately, used Navigate Stage, and verified the stage section becomes active and ready without requiring table readiness.", nil
}
