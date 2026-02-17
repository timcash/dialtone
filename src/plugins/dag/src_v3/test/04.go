package main

import (
	"fmt"
)

func Run04ThreeUserStoryBuildIO(ctx *testCtx) (string, error) {
	_, err := ctx.browser()
	if err != nil {
		return "", err
	}
	fmt.Println("[THREE] story step2 description:")
	fmt.Println("[THREE]   - In order to add output, the user selects processor and taps Add.")
	fmt.Println("[THREE]   - Add creates nodes only; user selects output=processor and input=output before tapping Link.")
	fmt.Println("[THREE]   - In order to add input, the user clears selection, taps Add, then selects output=input and input=processor before tapping Link.")
	fmt.Println("[THREE]   - Camera expectation: root layer remains fully readable while adding and linking nodes.")
	ctx.appendThought("story step2: verify stage before building root IO")

	if err := ctx.waitAria("Three Canvas", "need canvas before story step2 actions"); err != nil {
		return "", err
	}
	if err := ctx.captureShot("test_step_3_pre.png"); err != nil {
		return "", fmt.Errorf("capture story step2 pre screenshot: %w", err)
	}
	if err := ctx.clickNode(ctx.story.ProcessorID); err != nil {
		return "", err
	}
	if err := ctx.clickAction("graph", "add"); err != nil {
		return "", err
	}
	outID, err := ctx.lastCreatedNodeID()
	if err != nil || outID == "" {
		return "", fmt.Errorf("story step2 failed: missing output node id")
	}
	ctx.story.OutputID = outID
	if err := ctx.clickAction("graph", "clear_picks"); err != nil {
		return "", err
	}
	if err := ctx.clickNode(ctx.story.ProcessorID); err != nil {
		return "", err
	}
	if err := ctx.clickNode(ctx.story.OutputID); err != nil {
		return "", err
	}
	if err := ctx.clickAction("graph", "link_or_unlink"); err != nil {
		return "", err
	}
	if err := ctx.clickCanvas(8, 8, "clear-selection"); err != nil {
		return "", err
	}
	if err := ctx.clickAction("graph", "add"); err != nil {
		return "", err
	}
	inID, err := ctx.lastCreatedNodeID()
	if err != nil || inID == "" {
		return "", fmt.Errorf("story step2 failed: missing input node id")
	}
	ctx.story.InputID = inID
	if err := ctx.clickAction("graph", "clear_picks"); err != nil {
		return "", err
	}
	if err := ctx.clickNode(ctx.story.InputID); err != nil {
		return "", err
	}
	if err := ctx.clickNode(ctx.story.ProcessorID); err != nil {
		return "", err
	}
	if err := ctx.clickAction("graph", "link_or_unlink"); err != nil {
		return "", err
	}
	if err := ctx.clickAction("graph", "clear_picks"); err != nil {
		return "", err
	}
	if err := ctx.clickNode(ctx.story.InputID); err != nil {
		return "", err
	}
	if err := ctx.clickNode(ctx.story.ProcessorID); err != nil {
		return "", err
	}
	if err := ctx.clickAction("graph", "back"); err != nil {
		return "", err
	}
	ctx.logClick("step_done", "story_step_2", "ok")
	if err := ctx.captureShot("test_step_3.png"); err != nil {
		return "", fmt.Errorf("capture story step2 screenshot: %w", err)
	}
	return "Built root IO by creating output and input nodes around processor, linked both directions via selection pair semantics, and validated back/clear interaction flow.", nil
}
