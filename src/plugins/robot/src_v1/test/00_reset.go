package test

import (
	"time"

	chrome_app "dialtone/dev/plugins/chrome/app"
)

func Run00Reset(ctx *testCtx) (string, error) {
	_ = chrome_app.CleanupPort(3000)
	_ = chrome_app.CleanupPort(8080)
	time.Sleep(500 * time.Millisecond)
	return "Reset workspace: cleaned ports 3000/8080.", nil
}
