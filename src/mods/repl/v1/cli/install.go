package main

import (
	"fmt"
	"os"
)

func runInstall(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("repl install does not accept positional arguments")
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}

	cmd := nixDevelopCommand(repoRoot, "bash", "-lc", "command -v go >/dev/null && command -v gofmt >/dev/null")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("repl install failed: %w", err)
	}

	fmt.Println("repl v1 install complete")
	return nil
}
