package main

import (
	"fmt"
	"time"
)

func Run02Ps(ctx *testCtx) (string, error) {
	ctx.SetTimeout(30 * time.Second)
	
	// Interactive REPL test
	if err := ctx.StartREPL(); err != nil {
		return "", fmt.Errorf("failed to start REPL: %w", err)
	}
	// Cleanup on exit (kills REPL)
	defer ctx.Cleanup()

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
	if err := ctx.WaitForOutput("Started at", 5*time.Second); err != nil {
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

	// 6. Verify logs exist for the sleep command
	if err := ctx.WaitForLogEntry("subtone-", "Command: [proc sleep 10]", 10*time.Second); err != nil {
		return "", fmt.Errorf("failed to find sleep command in logs: %w", err)
	}
	
	// 7. Poll ps for cleanup
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if err := ctx.SendInput("ps"); err != nil {
			return "", err
		}
		if err := ctx.WaitForOutput("No active subtones", 2*time.Second); err == nil {
			return "Verified ps listed active subtones and confirmed cleanup.", nil
		}
		time.Sleep(1 * time.Second)
	}

	return "", fmt.Errorf("timed out waiting for subtones to cleanup")
}
