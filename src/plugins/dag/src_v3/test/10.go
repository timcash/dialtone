package main

import (
	"fmt"
)

func Run10ThreeUserStoryUnlinkAndRelabel(ctx *testCtx) (string, error) {
	_, err := ctx.browser()
	if err != nil {
		return "", err
	}
	fmt.Println("[THREE] story step7 description:")
	fmt.Println("[THREE]   - In order to remove edges, user selects output/input nodes and taps the context Link/Unlink button.")
	fmt.Println("[THREE]   - User clears selections between unlink actions.")
	fmt.Println("[THREE]   - User then renames processor again and expects camera to stay zoomed-out for full root readability.")

	if err := ctx.captureShot("test_step_8_pre.png"); err != nil {
		return "", fmt.Errorf("capture story step7 pre screenshot: %w", err)
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
	if err := ctx.clickAction("graph", "link_or_unlink"); err != nil {
		return "", err
	}
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
	if err := ctx.clickNode(ctx.story.ProcessorID); err != nil {
		return "", err
	}
	if err := ctx.renameSelected("Processor Final"); err != nil {
		return "", err
	}
	ctx.logClick("step_done", "story_step_7", "ok")
	if err := ctx.captureShot("test_step_8.png"); err != nil {
		return "", fmt.Errorf("capture story step7 screenshot: %w", err)
	}
	return "Unlinked input->processor and processor->output edges using context link/unlink action, then relabeled processor to final state.", nil
}
