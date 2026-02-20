package suite

import (
	"fmt"
)

func Run09ThreeUserStoryDeepCloseLayerHistory(ctx *testCtx) (string, error) {
	_, err := ctx.browser()
	if err != nil {
		return "", err
	}
	ctx.logf("STORY> step 6: open nested protocol layer on Link")

	if err := ctx.captureShot("test_step_7_pre.png"); err != nil {
		return "", fmt.Errorf("capture story step6 pre screenshot: %w", err)
	}
	if err := ctx.clickAction("graph", "back"); err != nil {
		return "", err
	}
	if err := ctx.clickAction("layer", "open_or_close_layer"); err != nil {
		return "", err
	}
	if err := ctx.assertHistoryDepthAtLeast(1); err != nil {
		return "", err
	}
	st, err := ctx.state()
	if err != nil {
		return "", err
	}
	if st.ActiveLayerID == "root" {
		return "", fmt.Errorf("expected nested layer active after open, got root")
	}
	if err := ctx.assertCameraAboveNode(ctx.story.LinkID, 8); err != nil {
		return "", err
	}
	ctx.logClick("step_done", "story_step_6", "ok")
	if err := ctx.captureShot("test_step_7.png"); err != nil {
		return "", fmt.Errorf("capture story step6 screenshot: %w", err)
	}
	return "Opened the nested protocol layer on `Link` with layer mode and verified nested-layer activation via unified logs.", nil
}
