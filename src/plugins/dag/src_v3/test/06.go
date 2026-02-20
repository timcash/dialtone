package test

import (
	"fmt"
)

func Run06ThreeUserStoryRenameAndCloseLayer(ctx *testCtx) (string, error) {
	_, err := ctx.browser()
	if err != nil {
		return "", err
	}
	ctx.logf("STORY> step 4: build Link -> Agent B")

	if err := ctx.captureShot("test_step_5_pre.png"); err != nil {
		return "", fmt.Errorf("capture story step4 pre screenshot: %w", err)
	}
	if err := ctx.clickAction("graph", "add"); err != nil {
		return "", err
	}
	bAgentID, err := ctx.lastCreatedNodeID()
	if err != nil || bAgentID == "" {
		return "", fmt.Errorf("step4 failed: missing Agent B id")
	}
	ctx.story.BAgentID = bAgentID
	if err := ctx.clickAction("graph", "link_or_unlink"); err != nil {
		return "", err
	}
	if err := ctx.renameSelectedNoModeSwitch("Agent B"); err != nil {
		return "", err
	}
	if err := ctx.assertProjectedInCanvas(ctx.story.BAgentID); err != nil {
		return "", err
	}
	if err := ctx.assertNodeCameraDistance(ctx.story.BAgentID, 27, 4); err != nil {
		return "", err
	}
	ctx.logClick("step_done", "story_step_4", "ok")
	if err := ctx.captureShot("test_step_5.png"); err != nil {
		return "", fmt.Errorf("capture story step4 screenshot: %w", err)
	}
	return "Created `Agent B`, linked `Link -> Agent B`, renamed it, and validated projection/camera logs.", nil
}
