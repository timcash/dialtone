package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func runBuild(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("chrome build does not accept positional arguments")
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	cliRoot, err := locateCliRoot(repoRoot)
	if err != nil {
		return err
	}

	binPath := filepath.Join(repoRoot, "bin", "chrome-v1")
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = cliRoot
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	fmt.Printf("built chrome v1 binary: %s\n", binPath)
	return nil
}
