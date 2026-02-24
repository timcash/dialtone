package test

func Run02ServerCheck(ctx *testCtx) (string, error) {
	if err := ctx.ensureSharedServer(); err != nil {
		return "", err
	}
	return "Go server running.", nil
}
