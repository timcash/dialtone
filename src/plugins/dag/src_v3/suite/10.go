package suite

import (
	"fmt"
)

func Run10ThreeUserStoryUnlinkAndRelabel(ctx *testCtx) (string, error) {
	_, err := ctx.browser()
	if err != nil {
		return "", err
	}
	ctx.logf("STORY> step 7: nested protocol Tx/Rx inside Link layer")

	if err := ctx.captureShot("test_step_8_pre.png"); err != nil {
		return "", fmt.Errorf("capture story step7 pre screenshot: %w", err)
	}
	if err := ctx.clickAction("layer", "add"); err != nil {
		return "", err
	}
	txID, err := ctx.lastCreatedNodeID()
	if err != nil || txID == "" {
		return "", fmt.Errorf("step7 failed: missing proto tx node")
	}
	ctx.story.ProtoTxID = txID
	if err := ctx.setRenameInput("Proto Tx"); err != nil {
		return "", err
	}
	if err := ctx.clickAction("layer", "rename"); err != nil {
		return "", err
	}
	if err := ctx.clickAction("layer", "add"); err != nil {
		return "", err
	}
	rxID, err := ctx.lastCreatedNodeID()
	if err != nil || rxID == "" {
		return "", fmt.Errorf("step7 failed: missing proto rx node")
	}
	ctx.story.ProtoRxID = rxID
	if err := ctx.clickAction("layer", "link_or_unlink"); err != nil {
		return "", err
	}
	if err := ctx.assertProjectedInCanvas(ctx.story.ProtoTxID); err != nil {
		return "", err
	}
	if err := ctx.assertProjectedInCanvas(ctx.story.ProtoRxID); err != nil {
		return "", err
	}
	ctx.logClick("step_done", "story_step_7", "ok")
	if err := ctx.captureShot("test_step_8.png"); err != nil {
		return "", fmt.Errorf("capture story step7 screenshot: %w", err)
	}
	return "Inside nested `Link` layer, created `Proto Tx` and `Proto Rx`, linked them, and validated projected node positions in viewport.", nil
}
