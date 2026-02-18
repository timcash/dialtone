package main

func Run11CleanupVerification(ctx *testCtx) (string, error) {
	if err := ctx.clickAction("layer", "back"); err != nil {
		return "", err
	}
	if err := ctx.clickAction("layer", "open_or_close_layer"); err != nil {
		return "", err
	}
	if err := ctx.assertActiveLayer("root"); err != nil {
		return "", err
	}
	return "Closed nested protocol layer back to `root` and verified the active-layer state via unified logs.", nil
}
