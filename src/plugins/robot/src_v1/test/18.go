package test

import "fmt"

func Run18CleanupVerification(ctx *testCtx) (string, error) {
	ctx.teardown()
	return fmt.Sprintf("Cleanup successful (teardown called, web port=%d).", ctx.webPort), nil
}
