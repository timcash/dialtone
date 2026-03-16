package main

import (
	"fmt"
	"os"
)

func runTest(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("repl test does not accept positional arguments")
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	srcRoot, err := locateSrcRoot(repoRoot)
	if err != nil {
		return err
	}

	cmd := nixDevelopCommand(repoRoot, "go", "test", "./mods/repl/v1/...")
	cmd.Dir = srcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("repl test failed: %w", err)
	}
	return nil
}
