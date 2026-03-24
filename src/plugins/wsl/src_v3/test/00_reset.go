package test

import (
	chrome_app "dialtone/dev/plugins/chrome/src_v1/go"
	"time"
)

func Run00Reset(ctx *testCtx) (string, error) {
	_ = chrome_app.CleanupPort(3000)
	_ = chrome_app.CleanupPort(ctx.webPort)
	time.Sleep(500 * time.Millisecond)
	return "Reset workspace: cleaned ports.", nil
}
