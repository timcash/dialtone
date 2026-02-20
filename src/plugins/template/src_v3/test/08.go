package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	test_v2 "dialtone/dev/plugins/dag/src_v3/suite"
)

func Run08UIRun() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	port, err := test_v2.PickFreePort()
	if err != nil {
		return err
	}

	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "template", "ui-run", "src_v3", "--port", fmt.Sprintf("%d", port))
	cmd.Dir = repoRoot
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
