package main

import (
	"fmt"
	"strings"
)

func Run02DevInstall(ctx *testCtx) (string, error) {
	// dev install might be slow, but it should skip if already installed
	output, err := ctx.runREPL("@DIALTONE dev install\nexit\n")
	if err != nil {
		return output, fmt.Errorf("dev install failed: %w", err)
	}

	required := []string{
		"DIALTONE> Installing latest Go runtime for managed ./dialtone.sh go commands...",
		"Go", // From "Go 1.26.0 already installed" or "Installing Go..."
		"DIALTONE> Bootstrap complete. Initializing dev.go scaffold...",
		"DIALTONE> Ready. You can now run plugin commands (install/build/test) via DIALTONE.",
	}

	for _, s := range required {
		if !strings.Contains(output, s) {
			return output, fmt.Errorf("missing expected output: %q", s)
		}
	}

	return output, nil
}
