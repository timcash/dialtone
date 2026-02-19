package main

import (
	"fmt"
	"strings"
)

func Run01Startup(ctx *testCtx) (string, error) {
	// Request help then exit
	output, err := ctx.runREPL("help\nexit\n")
	if err != nil {
		return output, fmt.Errorf("REPL failed: %w", err)
	}

	required := []string{
		"DIALTONE> Virtual Librarian online.",
		"Type 'help' for commands, or 'exit' to quit.",
		"USER-1> help",
		"DIALTONE> Help",
		"### Bootstrap",
		"`@DIALTONE dev install`",
		"### Plugins",
		"`@DIALTONE robot install src_v1`",
		"### System",
		"`<any command>`",
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
