package main

import (
	"fmt"
	"strings"
)

func Run04DagInstall(ctx *testCtx) (string, error) {
	// Send dag install and exit
	input := `dag install src_v3
exit
`
	output, err := ctx.runREPL(input)
	if err != nil {
		return output, fmt.Errorf("dag install failed: %w", err)
	}

	required := []string{
		"Request received. Spawning subtone for dag install...",
		"[DAG] Install: src_v3",
		"bun install",
		"Process", // ensure process exit is logged
		"exited with code 0",
	}

	for _, s := range required {
		if !strings.Contains(output, s) {
			return output, fmt.Errorf("missing expected output: %q", s)
		}
	}

	return output, nil
}
