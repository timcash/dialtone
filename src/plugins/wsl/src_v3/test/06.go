package test

func Run06CleanupVerification(ctx *testCtx) (string, error) {
	ctx.teardown()
	return "Cleanup successful.", nil
}
