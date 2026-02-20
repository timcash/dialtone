package test

import (
	"fmt"
	"time"
)

func Run03ThreeUserStoryStartEmpty(ctx *testCtx) (string, error) {
	_, err := ctx.browser()
	if err != nil {
		return "", err
	}
	ctx.logf("STORY> step 1: computer A program node")
	ctx.logf("STORY> user opens stage, adds first node, and names it Program A")
	if err := ctx.navigate(ctx.appURL("/#dag-3d-stage")); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: Three Canvas")
	if err := ctx.waitAria("Three Canvas", "need stage canvas before interactions"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: Three Canvas data-ready=true")
	if err := ctx.waitAriaAttrEquals("Three Canvas", "data-ready", "true", "wait for stage ready flag", 3*time.Second); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: DAG Mode aria label")
	if err := ctx.waitAria("DAG Mode", "need mode button"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: DAG Add aria label")
	if err := ctx.waitAria("DAG Add", "need add form action"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: DAG Label Input aria label")
	if err := ctx.waitAria("DAG Label Input", "need rename input"); err != nil {
		return "", err
	}
	if err := ctx.captureShot("test_step_2_pre.png"); err != nil {
		return "", fmt.Errorf("capture story step1 pre screenshot: %w", err)
	}
	if err := ctx.clickAction("graph", "add"); err != nil {
		return "", err
	}
	id, err := ctx.lastCreatedNodeID()
	if err != nil || id == "" {
		return "", fmt.Errorf("step1 failed: missing node id")
	}
	ctx.story.AProgramID = id
	if err := ctx.renameSelectedNoModeSwitch("Program A"); err != nil {
		return "", err
	}
	if err := ctx.assertProjectedInCanvas(ctx.story.AProgramID); err != nil {
		return "", err
	}
	if err := ctx.assertNodeCameraDistance(ctx.story.AProgramID, 27, 4); err != nil {
		return "", err
	}
	ctx.logClick("step_done", "story_step_1", "ok")
	if err := ctx.captureShot("test_step_2.png"); err != nil {
		return "", fmt.Errorf("capture story step1 screenshot: %w", err)
	}
	return "Created and labeled the root `Program A` node, then validated camera/node projection from unified logs.", nil
}
