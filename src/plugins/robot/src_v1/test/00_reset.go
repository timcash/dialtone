package test

import (
	"time"

	chrome_app "dialtone/dev/plugins/chrome/src_v1/go"
)

func Run00Reset(ctx *testCtx) (string, error) {
	_ = chrome_app.CleanupPort(3000)
	_ = chrome_app.CleanupPort(ctx.webPort)
	time.Sleep(500 * time.Millisecond)
	return "Reset workspace: cleaned ports 3000/test-web.", nil
}
