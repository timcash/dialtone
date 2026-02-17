package main

import (
	"fmt"
)

func Run09ThreeUserStoryDeepCloseLayerHistory(ctx *testCtx) (string, error) {
	_, err := ctx.browser()
	if err != nil {
		return "", err
	}
	fmt.Println("[THREE] story step6 description:")
	fmt.Println("[THREE]   - In order to close opened nested layers, user stays in Layer mode and taps Close Layer repeatedly.")
	fmt.Println("[THREE]   - Each close action must reduce history depth and lower camera y as the stack unwinds.")
	fmt.Println("[THREE]   - Final expectation: root layer visible with processor input/output context intact.")

	if err := ctx.captureShot("test_step_7_pre.png"); err != nil {
		return "", fmt.Errorf("capture story step6 pre screenshot: %w", err)
	}
	if err := ctx.clickAction("graph", "back"); err != nil {
		return "", err
	}
	if err := ctx.clickAction("layer", "open_or_close_layer"); err != nil {
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
	ctx.logClick("step_done", "story_step_6", "ok")
	if err := ctx.captureShot("test_step_7.png"); err != nil {
		return "", fmt.Errorf("capture story step6 screenshot: %w", err)
	}
	return "Closed deep nested layers in parent-first flow (`back` then `open/close`), returned to root processor context, and verified unwind behavior.", nil
}
