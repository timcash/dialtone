package main

import (
	"fmt"
	"time"
)

func Run02Ps(ctx *testCtx) (string, error) {
	// Interactive REPL test
	if err := ctx.StartREPL(); err != nil {
		return "", fmt.Errorf("failed to start REPL: %w", err)
	}
	defer ctx.Close()

	// Wait for prompt
	if err := ctx.WaitForOutput("USER-1>", 5*time.Second); err != nil {
		return "", err
	}

	// 1. Run proc test src_v1
	if err := ctx.SendInput("proc test src_v1"); err != nil {
		return "", err
	}

	// 2. Wait for confirmation that test started
	if err := ctx.WaitForOutput("Starting 3 parallel subtones", 5*time.Second); err != nil {
		return "", err
	}

	// 3. Wait for subtones to actually start (look for first subtone output)
	if err := ctx.WaitForOutput("Streaming stdout/stderr", 5*time.Second); err != nil {
		return "", err
	}

	// 4. Run ps
	if err := ctx.SendInput("ps"); err != nil {
		return "", err
	}

	// 5. Verify ps lists the processes
	if err := ctx.WaitForOutput("[proc sleep 10]", 5*time.Second); err != nil {
		return "", err
	}

	return "Verified ps listed active subtones from proc test src_v1", nil
}
