package test

func Run04DocsSectionValidation(ctx *testCtx) (string, error) {
	if err := ctx.navigateSection("docs"); err != nil {
		return "", err
	}
	if err := ctx.waitAria("WSL Documentation Section"); err != nil {
		return "", err
	}
	if err := ctx.captureShot("docs.png"); err != nil {
		return "", err
	}
	return "Docs section validated.", nil
}
