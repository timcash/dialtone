package main

import (
	"fmt"
)

func Run06ThreeUserStoryRenameAndCloseLayer(ctx *testCtx) (string, error) {
	_, err := ctx.browser()
	if err != nil {
		return "", err
	}
	fmt.Println("[THREE] story step4 description:")
	fmt.Println("[THREE]   - In order to change labels, the user selects node, types name in bottom textbox, and taps Rename.")
	fmt.Println("[THREE]   - In order to close an opened layer, the user switches to Layer mode and taps Close Layer.")
	fmt.Println("[THREE]   - Camera expectation: layer close moves camera to the parent node and updates history to zero.")

	if err := ctx.captureShot("test_step_5_pre.png"); err != nil {
		return "", fmt.Errorf("capture story step4 pre screenshot: %w", err)
	}
	if err := ctx.clickNode(ctx.story.NestedAID); err != nil {
		return "", err
	}
	if err := ctx.renameSelected("Nested Input"); err != nil {
		return "", err
	}
	if err := ctx.clickAction("graph", "back"); err != nil {
		return "", err
	}
	if err := ctx.clickAction("layer", "open_or_close_layer"); err != nil {
		return "", err
	}
	if err := ctx.clickNode(ctx.story.ProcessorID); err != nil {
		return "", err
	}
	if err := ctx.renameSelected("Processor"); err != nil {
		return "", err
	}
	ctx.logClick("step_done", "story_step_4", "ok")
	if err := ctx.captureShot("test_step_5.png"); err != nil {
		return "", fmt.Errorf("capture story step4 screenshot: %w", err)
	}
	return "Renamed nested node, backed out to parent layer, closed nested layer from parent context, and renamed processor in root context.", nil
}
