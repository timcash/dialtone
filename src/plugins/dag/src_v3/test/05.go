package main

import (
	"fmt"
)

func Run05ThreeUserStoryNestAndOpenLayer(ctx *testCtx) (string, error) {
	_, err := ctx.browser()
	if err != nil {
		return "", err
	}
	fmt.Println("[THREE] story step3 description:")
	fmt.Println("[THREE]   - In order to create/open a nested layer, the user selects processor, switches to Layer mode, and taps Open Layer.")
	fmt.Println("[THREE]   - After opening the layer, user builds nested nodes using Add, then links them explicitly.")
	fmt.Println("[THREE]   - Camera/layout expectation: nested layer anchors to parent x/z and elevates on +y; open-layer camera tracks that elevation.")

	if err := ctx.captureShot("test_step_4_pre.png"); err != nil {
		return "", fmt.Errorf("capture story step3 pre screenshot: %w", err)
	}
	if err := ctx.clickNode(ctx.story.ProcessorID); err != nil {
		return "", err
	}
	if err := ctx.clickAction("layer", "open_or_close_layer"); err != nil {
		return "", err
	}
	if err := ctx.clickAction("graph", "add"); err != nil {
		return "", err
	}
	n4, err := ctx.lastCreatedNodeID()
	if err != nil || n4 == "" {
		return "", fmt.Errorf("story step3 failed: missing nested node A")
	}
	ctx.story.NestedAID = n4
	if err := ctx.clickAction("graph", "add"); err != nil {
		return "", err
	}
	n5, err := ctx.lastCreatedNodeID()
	if err != nil || n5 == "" {
		return "", fmt.Errorf("story step3 failed: missing nested node B")
	}
	ctx.story.NestedBID = n5
	if err := ctx.clickAction("graph", "clear_picks"); err != nil {
		return "", err
	}
	if err := ctx.clickNode(ctx.story.NestedAID); err != nil {
		return "", err
	}
	if err := ctx.clickNode(ctx.story.NestedBID); err != nil {
		return "", err
	}
	if err := ctx.clickAction("graph", "link_or_unlink"); err != nil {
		return "", err
	}
	if err := ctx.clickNode(ctx.story.NestedAID); err != nil {
		return "", err
	}
	ctx.logClick("step_done", "story_step_3", "ok")
	if err := ctx.captureShot("test_step_4.png"); err != nil {
		return "", fmt.Errorf("capture story step3 screenshot: %w", err)
	}
	return "Opened processor nested layer, created two nested nodes, linked them, and preserved selection context inside the nested layer.", nil
}
