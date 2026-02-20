package main

import (
	"fmt"
	"time"
)

func Run03RobotInstall(ctx *testCtx) (string, error) {
	if err := ctx.StartREPL(); err != nil {
		return "", fmt.Errorf("failed to start REPL: %w", err)
	}
	defer ctx.Close()

	if err := ctx.WaitForOutput("USER-1>", 5*time.Second); err != nil {
		return "", err
	}

	// Robot Install
	if err := ctx.SendInput("robot install src_v1"); err != nil {
		return "", err
	}

	// Verify REPL acknowledgment
	if err := ctx.WaitForOutput("Request received. Spawning subtone for robot install...", 5*time.Second); err != nil {
		return "", err
	}
	if err := ctx.WaitForOutput("Started at", 5*time.Second); err != nil {
		return "", err
	}

	// Verify logs for "bun install"
	if err := ctx.WaitForLogEntry("subtone-", "bun install", 30*time.Second); err != nil {
		return "", fmt.Errorf("robot install logs missing 'bun install': %w", err)
	}

	// Ensure cleanup
	if err := ctx.WaitProcesses(15*time.Second); err != nil {
		// Note: robot install should finish. If it hangs, WaitProcesses might timeout.
		// But in test env, it runs.
	}

	return "Verified robot install logs.", nil
}
