package main

import (
	"fmt"
)

func Run08ThreeUserStoryDeepNestedBuild(ctx *testCtx) (string, error) {
	_, err := ctx.browser()
	if err != nil {
		return "", err
	}
	fmt.Println("[THREE] story step5 description:")
	fmt.Println("[THREE]   - In order to open an existing nested layer, user selects processor and taps Open Layer in Layer mode.")
	fmt.Println("[THREE]   - In order to create second-level nested layer, user selects nested node and taps Open Layer in Layer mode.")
	fmt.Println("[THREE]   - Camera/layout expectation: each deeper opened nested layer stacks higher on +y and camera y rises with depth.")

	if err := ctx.captureShot("test_step_6_pre.png"); err != nil {
		return "", fmt.Errorf("capture story step5 pre screenshot: %w", err)
	}
	if err := ctx.clickNode(ctx.story.ProcessorID); err != nil {
		return "", err
	}
	if err := ctx.clickAction("layer", "open_or_close_layer"); err != nil {
		return "", err
	}
	if err := ctx.clickNode(ctx.story.NestedBID); err != nil {
		return "", err
	}
	if err := ctx.clickAction("layer", "open_or_close_layer"); err != nil {
		return "", err
	}
	if err := ctx.clickAction("graph", "add"); err != nil {
		return "", err
	}
	n6, err := ctx.lastCreatedNodeID()
	if err != nil || n6 == "" {
		return "", fmt.Errorf("story step5 failed: missing level2 node A")
	}
	ctx.story.Level2AID = n6
	if err := ctx.clickAction("graph", "add"); err != nil {
		return "", err
	}
	n7, err := ctx.lastCreatedNodeID()
	if err != nil || n7 == "" {
		return "", fmt.Errorf("story step5 failed: missing level2 node B")
	}
	ctx.story.Level2BID = n7
	if err := ctx.clickAction("graph", "clear_picks"); err != nil {
		return "", err
	}
	if err := ctx.clickNode(ctx.story.Level2AID); err != nil {
		return "", err
	}
	if err := ctx.clickNode(ctx.story.Level2BID); err != nil {
		return "", err
	}
	if err := ctx.clickAction("graph", "link_or_unlink"); err != nil {
		return "", err
	}
	ctx.logClick("step_done", "story_step_5", "ok")
	if err := ctx.captureShot("test_step_6.png"); err != nil {
		return "", fmt.Errorf("capture story step5 screenshot: %w", err)
	}
	return "Re-opened processor nested layer, opened second-level nested layer, created deeper nodes, and linked them to validate multi-depth DAG interaction.", nil
}
