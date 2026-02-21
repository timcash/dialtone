package main

import (
	"fmt"
	"time"
)

func Run04DagInstall(ctx *testCtx) (string, error) {
	if err := ctx.StartREPL(); err != nil {
		return "", fmt.Errorf("failed to start REPL: %w", err)
	}
	defer ctx.Close()

	if err := ctx.WaitForOutput("USER-1>", 5*time.Second); err != nil {
		return "", err
	}

	// Dag Install
	if err := ctx.SendInput("dag install src_v3"); err != nil {
		return "", err
	}

	if err := ctx.WaitForOutput("Request received. Spawning subtone for dag install...", 5*time.Second); err != nil {
		return "", err
	}
	if err := ctx.WaitForOutput("Started at", 5*time.Second); err != nil {
		return "", err
	}

	// Verify logs
	if err := ctx.WaitForLogEntry("subtone-", "[DAG] Install: src_v3", 30*time.Second); err != nil {
		return "", fmt.Errorf("dag install logs missing signature: %w", err)
	}

	return "Verified dag install logs.", nil
}
