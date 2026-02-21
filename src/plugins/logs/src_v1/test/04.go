package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func Run04TwoProcessPingPong(ctx *testCtx) (string, error) {
	topic := "logs.pingpong.test"

	cmdA := exec.Command("./dialtone.sh", "logs", "pingpong", "src_v1",
		"--id", "alpha",
		"--peer", "beta",
		"--topic", topic,
		"--rounds", "3",
	)
	cmdA.Dir = ctx.repoRoot
	var outA bytes.Buffer
	cmdA.Stdout = &outA
	cmdA.Stderr = &outA
	if err := cmdA.Start(); err != nil {
		return "", fmt.Errorf("start alpha pingpong: %w", err)
	}

	time.Sleep(1 * time.Second)

	cmdB := exec.Command("./dialtone.sh", "logs", "pingpong", "src_v1",
		"--id", "beta",
		"--peer", "alpha",
		"--topic", topic,
		"--rounds", "3",
	)
	cmdB.Dir = ctx.repoRoot
	var outB bytes.Buffer
	cmdB.Stdout = &outB
	cmdB.Stderr = &outB
	if err := cmdB.Start(); err != nil {
		_ = cmdA.Process.Kill()
		return "", fmt.Errorf("start beta pingpong: %w", err)
	}

	waitWithTimeout := func(cmd *exec.Cmd, timeout time.Duration) error {
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case err := <-done:
			return err
		case <-time.After(timeout):
			_ = cmd.Process.Kill()
			return fmt.Errorf("timeout")
		}
	}

	if err := waitWithTimeout(cmdA, 20*time.Second); err != nil {
		return "", fmt.Errorf("alpha process failed: %v\n%s", err, outA.String())
	}
	if err := waitWithTimeout(cmdB, 20*time.Second); err != nil {
		return "", fmt.Errorf("beta process failed: %v\n%s", err, outB.String())
	}

	if !strings.Contains(outA.String(), "PINGPONG PASS") {
		return "", fmt.Errorf("alpha missing PASS marker:\n%s", outA.String())
	}
	if !strings.Contains(outB.String(), "PINGPONG PASS") {
		return "", fmt.Errorf("beta missing PASS marker:\n%s", outB.String())
	}

	return "Verified two dialtone logs processes exchanged 3 ping/pong rounds on one topic.", nil
}
