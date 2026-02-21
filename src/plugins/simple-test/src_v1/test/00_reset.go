package test

import (
	"time"

	chrome_app "dialtone/dev/plugins/chrome/src_v1/go"
)

func Run00Reset(ctx *testCtx) (string, error) {
	_ = chrome_app.CleanupPort(3000)
	time.Sleep(500 * time.Millisecond)
	return "Reset workspace: cleaned port 3000.", nil
}
