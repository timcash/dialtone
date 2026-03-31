package main

import (
	"fmt"
	"os"
	"time"

	test_v2 "dialtone/dev/plugins/test/src_v1/go"
)

func Run08UIRun() error {
	repoRoot, err := testRepoRoot()
	if err != nil {
		return err
	}

	port, err := test_v2.PickFreePort()
	if err != nil {
		return err
	}

	cmd, err := testDialtoneCommand(repoRoot, "cloudflare", "src_v1", "ui-run", "--port", fmt.Sprintf("%d", port))
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	return test_v2.WaitForPort(port, 12*time.Second)
}
