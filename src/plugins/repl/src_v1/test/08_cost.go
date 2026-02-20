package main

import (
	"fmt"
	"time"
)

func Run08CostLogs(ctx *testCtx) (string, error) {
	ctx.SetTimeout(20 * time.Second)

	if err := ctx.StartREPL(); err != nil {
		return "", fmt.Errorf("failed to start REPL: %w", err)
	}
	defer ctx.Cleanup()

	if err := ctx.WaitForOutput("USER-1>", 5*time.Second); err != nil {
		return "", err
	}

	cmd := "proc emit [COST] tokens=345k est_cost=0.003"
	if err := ctx.SendInput(cmd); err != nil {
		return "", err
	}

	if err := ctx.WaitForOutput("Started at", 5*time.Second); err != nil {
		return "", err
	}
	if err := ctx.WaitForOutput("[COST] tokens=345k est_cost=0.003", 8*time.Second); err != nil {
		return "", fmt.Errorf("cost line did not surface to root REPL: %w", err)
	}

	pid, err := ctx.ExtractLastSubtonePID()
	if err != nil {
		return "", err
	}
	logPattern := fmt.Sprintf("subtone-%s-", pid)
	if err := ctx.WaitForLogEntry(logPattern, "[COST] tokens=345k est_cost=0.003", 8*time.Second); err != nil {
		return "", fmt.Errorf("cost line did not reach subtone log file: %w", err)
	}

	return "Verified [COST] lines are forwarded to root REPL and written in subtone logs.", nil
}
