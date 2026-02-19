package main

import (
	"fmt"
	"strings"
)

func Run01Startup(ctx *testCtx) (string, error) {
	output, err := ctx.runREPL("exit\n")
	if err != nil {
		return output, fmt.Errorf("REPL failed: %w", err)
	}

	required := []string{
		"DIALTONE> Virtual Librarian online.",
		"I can bootstrap dev tools, route commands through dev.go, and help install plugins.",
		"Type 'help' for commands, or 'exit' to quit.",
		"USER-1> exit",
		"DIALTONE> Goodbye.",
	}

	for _, s := range required {
		if !strings.Contains(output, s) {
			return output, fmt.Errorf("missing expected output: %q", s)
		}
	}

	return output, nil
}
