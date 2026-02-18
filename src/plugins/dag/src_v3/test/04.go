package main

import (
	"fmt"
)

func Run04ThreeUserStoryBuildIO(ctx *testCtx) (string, error) {
	_, err := ctx.browser()
	if err != nil {
		return "", err
	}
	ctx.logf("STORY> step 2: build Program A -> Agent A")

	if err := ctx.waitAria("Three Canvas", "need canvas before story step2 actions"); err != nil {
		return "", err
	}
	if err := ctx.captureShot("test_step_3_pre.png"); err != nil {
		return "", fmt.Errorf("capture story step2 pre screenshot: %w", err)
	}
	if err := ctx.clickAction("graph", "add"); err != nil {
		return "", err
	}
	agentID, err := ctx.lastCreatedNodeID()
	if err != nil || agentID == "" {
		return "", fmt.Errorf("step2 failed: missing Agent A id")
	}
	ctx.story.AAgentID = agentID
	if err := ctx.clickAction("graph", "link_or_unlink"); err != nil {
		return "", err
	}
	if err := ctx.renameSelectedNoModeSwitch("Agent A"); err != nil {
		return "", err
	}
	if err := ctx.assertProjectedInCanvas(ctx.story.AAgentID); err != nil {
		return "", err
	}
	if err := ctx.assertNodeCameraDistance(ctx.story.AAgentID, 27, 4); err != nil {
		return "", err
	}
	ctx.logClick("step_done", "story_step_2", "ok")
	if err := ctx.captureShot("test_step_3.png"); err != nil {
		return "", fmt.Errorf("capture story step2 screenshot: %w", err)
	}
	return "Created `Agent A`, linked `Program A -> Agent A`, renamed it, and validated projection/camera logs.", nil
}
