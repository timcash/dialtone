package test

import (
	"fmt"
	"time"
)

func Run02NoBackendMenuToStage(ctx *testCtx) (string, error) {
	ctx.ensureBackendStopped()
	ctx.setRequireBackend(false)
	defer ctx.setRequireBackend(true)
	if _, err := ctx.browser(); err != nil {
		return "", err
	}

	ctx.appendThought("no-backend startup: force backend down and load dev app")
	if err := ctx.runEval(`(() => {
		try {
			window.sessionStorage.removeItem('dag.src_v3.active_section');
			window.sessionStorage.removeItem('dag.src_v3.api_ready');
		} catch {}
		return true;
	})()`, nil); err != nil {
		return "", fmt.Errorf("clear no-backend startup session state: %w", err)
	}

	ctx.logf("LOOKING FOR: dev server at %s", ctx.devURL("/"))
	if err := ctx.waitHTTPReady(ctx.devURL("/"), 12*time.Second); err != nil {
		return "", fmt.Errorf("dev startup wait failed: %w", err)
	}
	ctx.logf("LOOKING FOR: navigation to %s", ctx.devURL("/"))
	if err := ctx.navigate(ctx.devURL("/")); err != nil {
		return "", fmt.Errorf("no-backend app navigate failed: %w", err)
	}

	ctx.logf("LOOKING FOR: App Header data-boot=true")
	if err := ctx.waitAriaAttrEquals("App Header", "data-boot", "true", "wait for app boot", 10*time.Second); err != nil {
		return "", err
	}

	ctx.logf("LOOKING FOR: dag-3d-stage data-ready=true")
	if err := ctx.waitAriaAttrEquals("Three Section", "data-ready", "true", "wait for stage section ready", 10*time.Second); err != nil {
		return "", err
	}

	ctx.logf("LOOKING FOR: Toggle Global Menu button")
	if err := ctx.waitAria("Toggle Global Menu", "no-backend startup needs menu toggle"); err != nil {
		return "", err
	}
	if err := ctx.captureShot("test_step_no_backend_menu_stage_pre.png"); err != nil {
		return "", fmt.Errorf("capture no-backend pre screenshot: %w", err)
	}
	ctx.logf("LOOKING FOR: Global Menu Panel after toggle")
	if err := ctx.clickAria("Toggle Global Menu", "open global menu without backend"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: Navigate Stage button")
	if err := ctx.waitAria("Navigate Stage", "no-backend startup needs stage nav button"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: Three Canvas after section switch")
	if err := ctx.clickAria("Navigate Stage", "switch to stage without backend"); err != nil {
		return "", err
	}
	if err := ctx.waitAria("Three Canvas", "stage canvas should exist without backend"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: Three Section data-active=true")
	if err := ctx.waitAriaAttrEquals("Three Section", "data-active", "true", "stage section should be active without backend", 6*time.Second); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: Three Canvas data-ready=true")
	if err := ctx.waitAriaAttrEquals("Three Canvas", "data-ready", "true", "stage should report ready without backend", 8*time.Second); err != nil {
		return "", err
	}
	if err := ctx.captureShot("test_step_no_backend_menu_stage.png"); err != nil {
		return "", fmt.Errorf("capture no-backend stage screenshot: %w", err)
	}

	return "With backend unavailable, loaded dev app, opened menu, navigated to Stage, and verified stage section becomes active and ready.", nil
}
