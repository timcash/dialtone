package test

import (
	"os"
	"path/filepath"
	"time"

	chrome_app "dialtone/dev/plugins/chrome/app"
)

func Run00Reset(ctx *testCtx) (string, error) {
	// 1. Kill any existing servers on our ports
	_ = chrome_app.CleanupPort(3000)
	_ = chrome_app.CleanupPort(8080)
	time.Sleep(500 * time.Millisecond)

	// 2. Clear UI dist to ensure fresh build
	repoRoot, err := findRepoRoot()
	if err == nil {
		dist := filepath.Join(repoRoot, "src", "plugins", "dag", "src_v3", "ui", "dist")
		_ = os.RemoveAll(dist)
	}

	return "Reset workspace: cleaned ports 3000/8080 and removed UI dist for fresh build.", nil
}
