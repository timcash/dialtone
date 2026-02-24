package test

func Run03HomeSectionValidation(ctx *testCtx) (string, error) {
	if _, err := ctx.ensureSharedBrowser(); err != nil {
		return "", err
	}
	if err := ctx.navigateSection("home"); err != nil {
		return "", err
	}
	if err := ctx.waitAria("WSL Hero Section"); err != nil {
		return "", err
	}
	if err := ctx.captureShot("home.png"); err != nil {
		return "", err
	}
	return "Hero section validated.", nil
}
