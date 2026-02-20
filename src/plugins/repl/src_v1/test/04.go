package main

import (
	"fmt"
	"strings"
)

func Run04DagInstall(ctx *testCtx) (string, error) {
	// Send dag install and exit
	input := `@DIALTONE dag install src_v3
exit
`
	output, err := ctx.runREPL(input)
	if err != nil {
		return output, fmt.Errorf("dag install failed: %w", err)
	}

	required := []string{
		"Request received. Spawning subtone for dag install...",
		">> [DAG] Install: src_v3",
		">> [DAG] Install complete: src_v3",
	}

	for _, s := range required {
		if !strings.Contains(output, s) {
			return output, fmt.Errorf("missing expected output: %q", s)
		}
	}

	return output, nil
}
