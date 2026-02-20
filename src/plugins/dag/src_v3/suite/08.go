package suite

import (
	"fmt"
)

func Run08ThreeUserStoryDeepNestedBuild(ctx *testCtx) (string, error) {
	_, err := ctx.browser()
	if err != nil {
		return "", err
	}
	ctx.logf("STORY> step 5: build Agent B -> Program B")

	if err := ctx.captureShot("test_step_6_pre.png"); err != nil {
		return "", fmt.Errorf("capture story step5 pre screenshot: %w", err)
	}
	if err := ctx.clickAction("graph", "add"); err != nil {
		return "", err
	}
	bProgramID, err := ctx.lastCreatedNodeID()
	if err != nil || bProgramID == "" {
		return "", fmt.Errorf("step5 failed: missing Program B id")
	}
	ctx.story.BProgramID = bProgramID
	if err := ctx.clickAction("graph", "link_or_unlink"); err != nil {
		return "", err
	}
	if err := ctx.renameSelectedNoModeSwitch("Program B"); err != nil {
		return "", err
	}
	if err := ctx.assertProjectedInCanvas(ctx.story.BProgramID); err != nil {
		return "", err
	}
	if err := ctx.assertNodeCameraDistance(ctx.story.BProgramID, 27, 4); err != nil {
		return "", err
	}
	ctx.logClick("step_done", "story_step_5", "ok")
	if err := ctx.captureShot("test_step_6.png"); err != nil {
		return "", fmt.Errorf("capture story step5 screenshot: %w", err)
	}
	return "Created `Program B`, linked `Agent B -> Program B`, renamed it, and validated projection/camera logs.", nil
}
