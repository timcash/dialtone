package test

func Run05TableSectionValidation(ctx *testCtx) (string, error) {
	if err := ctx.navigateSection("table"); err != nil {
		return "", err
	}
	if err := ctx.waitAria("WSL Spreadsheet Section"); err != nil {
		return "", err
	}
	if err := ctx.captureShot("table.png"); err != nil {
		return "", err
	}
	return "Table section validated.", nil
}
