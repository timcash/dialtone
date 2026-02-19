package main

import (
	"fmt"
	"strings"
)

func Run03RobotInstall(ctx *testCtx) (string, error) {
	// Send robot install and exit
	input := "@DIALTONE robot install src_v1\nexit\n"
	output, err := ctx.runREPL(input)
	if err != nil {
		return output, fmt.Errorf("robot install failed: %w", err)
	}

	required := []string{
		"Request received. Spawning subtone for robot install...",
		">> [Robot] Install: src_v1",
		">> [Robot] Install complete: src_v1",
	}

	for _, s := range required {
		if !strings.Contains(output, s) {
			return output, fmt.Errorf("missing expected output: %q", s)
		}
	}

	return output, nil
}
