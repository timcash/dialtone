package main

import (
	"fmt"
	"time"
)

func Run02DevInstall(ctx *testCtx) (string, error) {
	if err := ctx.StartREPL(); err != nil {
		return "", fmt.Errorf("failed to start REPL: %w", err)
	}
	defer ctx.Close()

	if err := ctx.WaitForOutput("USER-1>", 5*time.Second); err != nil {
		return "", err
	}

	// Dev Install
	// Note: dev install runs a subtone named "__bootstrap_dev" or similar if implemented as subtone?
	// In dev.go:
	// if len(args) > 0 && args[0] == "install" { runDevInstall(); return }
	// This runs IN PROCESS, not as subtone.
	// But REPL `dev install` handling?
	// startREPL doesn't have special case for "dev install".
	// It parses "dev install" -> args ["dev", "install"].
	// Calls proc.RunSubtone(["dev", "install"]).
	// Subtone runs `./dialtone.sh dev install`.
	// `./dialtone.sh` runs `go run dev.go dev install`.
	// `dev.go` `main()` calls `runDevInstall()`.
	// `runDevInstall` prints to stdout.
	// Subtone captures stdout to LOG FILE.
	// So "dev install" output is HIDDEN in REPL.
	
	if err := ctx.SendInput("dev install"); err != nil {
		return "", err
	}

	if err := ctx.WaitForOutput("Request received. Spawning subtone for dev install", 5*time.Second); err != nil {
		return "", err
	}
	if err := ctx.WaitForOutput("Started at", 5*time.Second); err != nil {
		return "", err
	}

	// Verify logs
	if err := ctx.WaitForLogEntry("subtone-", "Installing latest Go runtime", 30*time.Second); err != nil {
		return "", fmt.Errorf("dev install logs missing signature: %w", err)
	}

	return "Verified dev install logs.", nil
}
