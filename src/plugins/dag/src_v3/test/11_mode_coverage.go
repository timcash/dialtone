package test

import "fmt"

func Run11SwitchToBuildMode(ctx *testCtx) (string, error) {
	ctx.logf("LOOKING FOR: mode switch to graph")
	if err := ctx.ensureMode("graph"); err != nil {
		return "", err
	}
	if err := ctx.assertMode("graph"); err != nil {
		return "", err
	}
	return "Confirmed the form calculator is in Build mode.", nil
}

func Run11BuildModeCoverageA(ctx *testCtx) (string, error) {
	ctx.logf("LOOKING FOR: action Add")
	if err := ctx.clickAction("graph", "add"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Link/Unlink")
	if err := ctx.clickAction("graph", "link_or_unlink"); err != nil {
		return "", err
	}
	if err := ctx.setRenameInput("Build Relay"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Rename")
	if err := ctx.clickAction("graph", "rename"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Focus")
	if err := ctx.clickAction("graph", "focus"); err != nil {
		return "", err
	}
	id, err := ctx.lastCreatedNodeID()
	if err != nil || id == "" {
		return "", fmt.Errorf("build coverage A missing created node id")
	}
	if err := ctx.assertProjectedInCanvas(id); err != nil {
		return "", err
	}
	return "Build mode used `Add`, `Link/Unlink`, `Rename`, and `Focus` with viewport projection validation.", nil
}

func Run11BuildModeCoverageB(ctx *testCtx) (string, error) {
	ctx.logf("LOOKING FOR: action Open/Close")
	if err := ctx.clickAction("graph", "open_or_close_layer"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Back")
	if err := ctx.clickAction("graph", "back"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Toggle Labels")
	if err := ctx.clickAction("graph", "toggle_labels"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Clear Picks")
	if err := ctx.clickAction("graph", "clear_picks"); err != nil {
		return "", err
	}
	if err := ctx.assertActiveLayer("root"); err != nil {
		return "", err
	}
	return "Build mode used `Open/Close`, `Back`, `Labels`, and `Clear`, and verified root-layer state.", nil
}

func Run12SwitchToLayerMode(ctx *testCtx) (string, error) {
	ctx.logf("LOOKING FOR: mode switch to layer")
	if err := ctx.ensureMode("layer"); err != nil {
		return "", err
	}
	if err := ctx.assertMode("layer"); err != nil {
		return "", err
	}
	return "Confirmed the form calculator is in Layer mode.", nil
}

func Run13LayerModeCoverageA(ctx *testCtx) (string, error) {
	ctx.logf("LOOKING FOR: action Add (Layer)")
	if err := ctx.clickAction("layer", "add"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Link/Unlink (Layer)")
	if err := ctx.clickAction("layer", "link_or_unlink"); err != nil {
		return "", err
	}
	if err := ctx.setRenameInput("Layer Relay"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Rename (Layer)")
	if err := ctx.clickAction("layer", "rename"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Focus (Layer)")
	if err := ctx.clickAction("layer", "focus"); err != nil {
		return "", err
	}
	id, err := ctx.lastCreatedNodeID()
	if err != nil || id == "" {
		return "", fmt.Errorf("layer coverage A missing created node id")
	}
	if err := ctx.assertProjectedInCanvas(id); err != nil {
		return "", err
	}
	return "Layer mode used `Add`, `Link/Unlink`, `Rename`, and `Focus` on a relay node with viewport projection validation.", nil
}

func Run14LayerModeCoverageB(ctx *testCtx) (string, error) {
	ctx.logf("LOOKING FOR: action Open/Close (Layer)")
	if err := ctx.clickAction("layer", "open_or_close_layer"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Back (Layer)")
	if err := ctx.clickAction("layer", "back"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Toggle Labels (Layer)")
	if err := ctx.clickAction("layer", "toggle_labels"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Clear Picks (Layer)")
	if err := ctx.clickAction("layer", "clear_picks"); err != nil {
		return "", err
	}
	if err := ctx.assertActiveLayer("root"); err != nil {
		return "", err
	}
	return "Layer mode used `Open/Close`, `Back`, `Labels`, and `Clear`, and confirmed return to the root layer.", nil
}

func Run15SwitchToCameraMode(ctx *testCtx) (string, error) {
	ctx.logf("LOOKING FOR: mode switch to camera")
	if err := ctx.clickAria("DAG Mode", "switch layer controls to view controls"); err != nil {
		return "", err
	}
	if err := ctx.assertMode("camera"); err != nil {
		return "", err
	}
	return "Switched the form calculator from Layer mode to View mode.", nil
}

func Run16CameraModeCoverageA(ctx *testCtx) (string, error) {
	ctx.logf("LOOKING FOR: action Top View")
	if err := ctx.clickAction("camera", "camera_top"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Iso View")
	if err := ctx.clickAction("camera", "camera_iso"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Side View")
	if err := ctx.clickAction("camera", "camera_side"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Focus (Camera)")
	if err := ctx.clickAction("camera", "focus"); err != nil {
		return "", err
	}
	return "View mode used camera orientation controls `Top`, `Iso`, `Side`, then `Focus` to reframe the active layer.", nil
}

func Run17CameraModeCoverageB(ctx *testCtx) (string, error) {
	ctx.logf("LOOKING FOR: action Pan Left")
	if err := ctx.clickAction("camera", "camera_left"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Pan Up")
	if err := ctx.clickAction("camera", "camera_up"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Pan Right")
	if err := ctx.clickAction("camera", "camera_right"); err != nil {
		return "", err
	}
	ctx.logf("LOOKING FOR: action Pan Down")
	if err := ctx.clickAction("camera", "camera_down"); err != nil {
		return "", err
	}
	return "View mode used stateful camera offset controls `Left`, `Up`, `Right`, and `Down`.", nil
}

func Run18FinalizeAndTeardown(ctx *testCtx) (string, error) {
	ctx.logf("LOOKING FOR: mode switch back to graph")
	if err := ctx.clickAria("DAG Mode", "return to build mode before teardown"); err != nil {
		return "", err
	}
	if err := ctx.assertMode("graph"); err != nil {
		return "", err
	}
	ctx.teardown()
	return "Returned controls to Build mode and tore down shared browser/backend resources.", nil
}
