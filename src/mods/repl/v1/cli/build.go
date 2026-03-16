package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func runBuild(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("repl build does not accept positional arguments")
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	srcRoot, err := locateSrcRoot(repoRoot)
	if err != nil {
		return err
	}

	binPath := filepath.Join(repoRoot, "bin", "repl-v1")
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}

	cmd := nixDevelopCommand(repoRoot, "go", "build", "-o", filepath.Join("..", "bin", "repl-v1"), "./mods/repl/v1")
	cmd.Dir = srcRoot
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("repl build failed: %w", err)
	}

	fmt.Printf("built repl v1 binary: %s\n", binPath)
	return nil
}
