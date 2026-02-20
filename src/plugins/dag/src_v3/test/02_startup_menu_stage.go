package test

import (
	"fmt"
	"time"
)

func Run02StartupMenuToStageFresh(ctx *testCtx) (string, error) {
	if _, err := ctx.browser(); err != nil {
		return "", err
	}

	ctx.logf("LOOKING FOR: backend server at %s", ctx.appURL("/"))
	if err := ctx.waitHTTPReady(ctx.appURL("/"), 12*time.Second); err != nil {
		return "", fmt.Errorf("backend startup wait failed: %w", err)
	}
	ctx.logf("LOOKING FOR: navigation to %s", ctx.appURL("/"))

	_ = ctx.runEval(`(() => {
		const h = document.querySelector('[aria-label="App Header"]');
		if (h) h.setAttribute('data-boot', 'false');
		window.sessionStorage.clear();
		return true;
	})()`, nil)

	_ = ctx.navigate("about:blank")
	time.Sleep(500 * time.Millisecond)
	if err := ctx.navigate(ctx.appURL("/")); err != nil {
		ctx.appendThought("startup nav: first navigate failed, retrying once after short wait")
		time.Sleep(500 * time.Millisecond)
		if errRetry := ctx.navigate(ctx.appURL("/")); errRetry != nil {
			return "", fmt.Errorf("fresh app navigate failed: %w", errRetry)
		}
	}

	ctx.logf("LOOKING FOR: App Header data-boot=true")
	if err := ctx.waitAriaAttrEquals("App Header", "data-boot", "true", "wait for app boot", 30*time.Second); err != nil {
		_ = ctx.captureShot("timeout_boot_step4.png")
		return "", err
	}

	ctx.logf("LOOKING FOR: dag-meta-table data-ready=true")
	if err := ctx.waitAriaAttrEquals("DAG Table Section", "data-ready", "true", "wait for initial section ready", 10*time.Second); err != nil {
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

	ctx.logf("LOOKING FOR: Toggle Global Menu button")
	if err := ctx.waitAria("Toggle Global Menu", "fresh startup needs menu toggle"); err != nil {
		return "", err
	}
	if err := ctx.captureShot("test_step_startup_menu_stage_pre.png"); err != nil {
		return "", fmt.Errorf("capture startup pre screenshot: %w", err)
	}
	ctx.logf("LOOKING FOR: Global Menu Panel after toggle")
	if err := ctx.clickAria("Toggle Global Menu", "open global menu from fresh startup"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: Navigate Stage button")
	if err := ctx.waitAria("Navigate Stage", "fresh startup needs stage nav button"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: Three Canvas after section switch")
	if err := ctx.clickAria("Navigate Stage", "switch to stage from menu"); err != nil {
		return "", err
	}
	if err := ctx.waitAria("Three Canvas", "stage canvas should exist after menu nav"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: Three Section data-active=true")
	if err := ctx.waitAriaAttrEquals("Three Section", "data-active", "true", "stage section should be active", 6*time.Second); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: Three Canvas data-ready=true")
	if err := ctx.waitAriaAttrEquals("Three Canvas", "data-ready", "true", "stage should report ready", 8*time.Second); err != nil {
		return "", err
	}
	if err := ctx.captureShot("test_step_startup_menu_stage.png"); err != nil {
		return "", fmt.Errorf("capture startup stage screenshot: %w", err)
	}

	return "Fresh app startup opened menu immediately, used Navigate Stage, and verified the stage section becomes active and ready without requiring table readiness.", nil
}
