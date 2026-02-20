package test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	test_v2 "dialtone/dev/plugins/test/src_v1/go"
)

func Run08UIRun(ctx *testCtx) (string, error) {
	port, err := test_v2.PickFreePort()
	if err != nil {
		return "", err
	}

	cmd := exec.Command(filepath.Join(ctx.repoRoot, "dialtone.sh"), "robot", "ui-run", "src_v1", "--port", fmt.Sprintf("%d", port))
	cmd.Dir = ctx.repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return "", err
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	if err := test_v2.WaitForPort(port, 12*time.Second); err != nil {
		return "", err
	}
	return "UI server check passed.", nil
}
