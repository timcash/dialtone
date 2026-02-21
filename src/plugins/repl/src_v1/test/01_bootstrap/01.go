package main

import (
	"fmt"
	"time"
)

func Run01Startup(ctx *testCtx) (string, error) {
	// Interactive REPL test
	if err := ctx.StartREPL(); err != nil {
		return "", fmt.Errorf("failed to start REPL: %w", err)
	}
	defer ctx.Close()

	// Wait for prompt
	if err := ctx.WaitForOutput("USER-1>", 5*time.Second); err != nil {
		return "", err
	}

	// 1. Help
	if err := ctx.SendInput("help"); err != nil {
		return "", err
	}

	required := []string{
		"DIALTONE> Help",
		"### Bootstrap",
		"`@DIALTONE dev install`",
		"### Plugins",
		"`robot install src_v1`",
		"`dag install src_v3`",
		"### System",
		"`ps`",
		"`<any command>`",
	}

	for _, s := range required {
		if err := ctx.WaitForOutput(s, 5*time.Second); err != nil {
			return "", fmt.Errorf("missing expected output: %q", s)
		}
	}

	return "Verified REPL startup and help command.", nil
}
