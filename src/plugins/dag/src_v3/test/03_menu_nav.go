package main

import (
	"fmt"
	"time"
)

func Run03MenuNavSectionSwitch(ctx *testCtx) (string, error) {
	_, err := ctx.browser()
	if err != nil {
		return "", err
	}

	if err := ctx.navigate("http://127.0.0.1:8080/#dag-table"); err != nil {
		return "", fmt.Errorf("menu nav section switch failed: %w", err)
	}
	ctx.appendThought("menu nav: wait for table and open menu")
	if err := ctx.waitAria("Toggle Global Menu", "need menu toggle"); err != nil {
		return "", err
	}
	if err := ctx.waitAria("DAG Table", "need table visible"); err != nil {
		return "", err
	}
	if err := ctx.waitAriaAttrEquals("DAG Table", "data-ready", "true", "wait for table ready", 8*time.Second); err != nil {
		return "", err
	}
	if err := ctx.clickAria("Toggle Global Menu", "open global menu"); err != nil {
		return "", err
	}
	if err := ctx.waitAria("Navigate Stage", "need stage menu button"); err != nil {
		return "", err
	}
	if err := ctx.captureShot("test_step_menu_nav_pre.png"); err != nil {
		return "", fmt.Errorf("capture menu nav pre screenshot: %w", err)
	}
	if err := ctx.clickAria("Navigate Stage", "switch section to stage"); err != nil {
		return "", err
	}
	if err := ctx.waitAria("Three Canvas", "confirm stage visible after nav"); err != nil {
		return "", err
	}
	if err := ctx.waitAriaAttrEquals("Three Canvas", "data-ready", "true", "wait for stage ready after nav", 6*time.Second); err != nil {
		return "", err
	}
	if err := ctx.captureShot("test_step_menu_nav.png"); err != nil {
		return "", fmt.Errorf("capture menu nav screenshot: %w", err)
	}

	return "Opened global menu from table, navigated to stage through menu action, and verified the stage canvas becomes ready after section switch.", nil
}
