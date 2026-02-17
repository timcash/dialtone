package main

import (
	"fmt"
	"time"
)

func Run03ThreeUserStoryStartEmpty(ctx *testCtx) (string, error) {
	_, err := ctx.browser()
	if err != nil {
		return "", err
	}
	fmt.Println("[THREE] story step1 description:")
	fmt.Println("[THREE]   - In order to create a new node, the user taps Add.")
	fmt.Println("[THREE]   - The user starts from an empty DAG in root layer and expects one selected node after add.")
	fmt.Println("[THREE]   - Camera expectation: zoomed-out root framing with room for upcoming input/output nodes.")
	ctx.appendThought("story step1: load stage and verify controls are visible")
	if err := ctx.navigate("http://127.0.0.1:8080/#three"); err != nil {
		return "", err
	}
	if err := ctx.waitAria("Three Canvas", "need stage canvas before interactions"); err != nil {
		return "", err
	}
	if err := ctx.waitAriaAttrEquals("Three Canvas", "data-ready", "true", "wait for stage ready flag", 3*time.Second); err != nil {
		return "", err
	}
	if err := ctx.waitAria("DAG Mode", "need mode button"); err != nil {
		return "", err
	}
	if err := ctx.waitAria("DAG Thumb 1", "need thumb 1"); err != nil {
		return "", err
	}
	if err := ctx.waitAria("DAG Thumb 2", "need thumb 2"); err != nil {
		return "", err
	}
	if err := ctx.waitAria("DAG Thumb 3", "need thumb 3"); err != nil {
		return "", err
	}
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
		return "", fmt.Errorf("story step1 failed: missing processor id")
	}
	ctx.story.ProcessorID = id
	if err := ctx.clickAction("camera", "camera_top"); err != nil {
		return "", err
	}
	if err := ctx.clickAction("camera", "camera_side"); err != nil {
		return "", err
	}
	if err := ctx.clickAction("camera", "camera_iso"); err != nil {
		return "", err
	}
	if err := ctx.clickNode(ctx.story.ProcessorID); err != nil {
		return "", err
	}
	ctx.logClick("step_done", "story_step_1", "ok")
	if err := ctx.captureShot("test_step_2.png"); err != nil {
		return "", fmt.Errorf("capture story step1 screenshot: %w", err)
	}
	return "Loaded the stage controls, added the first node, cycled camera modes (top/side/iso), and reselected the new node to verify interaction readiness.", nil
}
