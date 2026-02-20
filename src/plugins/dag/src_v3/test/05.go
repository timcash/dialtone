package test

import (
	"fmt"
)

func Run05ThreeUserStoryNestAndOpenLayer(ctx *testCtx) (string, error) {
	_, err := ctx.browser()
	if err != nil {
		return "", err
	}
	ctx.logf("STORY> step 3: build Agent A -> Link")

	if err := ctx.captureShot("test_step_4_pre.png"); err != nil {
		return "", fmt.Errorf("capture story step3 pre screenshot: %w", err)
	}
	if err := ctx.clickAction("graph", "add"); err != nil {
		return "", err
	}
	linkID, err := ctx.lastCreatedNodeID()
	if err != nil || linkID == "" {
		return "", fmt.Errorf("step3 failed: missing Link id")
	}
	ctx.story.LinkID = linkID
	if err := ctx.clickAction("graph", "link_or_unlink"); err != nil {
		return "", err
	}
	if err := ctx.renameSelectedNoModeSwitch("Link"); err != nil {
		return "", err
	}
	if err := ctx.assertProjectedInCanvas(ctx.story.LinkID); err != nil {
		return "", err
	}
	if err := ctx.assertNodeCameraDistance(ctx.story.LinkID, 27, 4); err != nil {
		return "", err
	}
	ctx.logClick("step_done", "story_step_3", "ok")
	if err := ctx.captureShot("test_step_4.png"); err != nil {
		return "", fmt.Errorf("capture story step3 screenshot: %w", err)
	}
	return "Created `Link`, linked `Agent A -> Link`, renamed it, and validated projection/camera logs.", nil
}
