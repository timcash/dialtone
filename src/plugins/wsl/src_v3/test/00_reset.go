package test

import (
	"time"
)

func Run00Reset(ctx *testCtx) (string, error) {
	_ = cleanupPort(3000)
	_ = cleanupPort(ctx.webPort)
	time.Sleep(500 * time.Millisecond)
	return "Reset workspace: cleaned ports.", nil
}
