package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func runBuild(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("mod build does not accept positional arguments")
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	modRoot, err := locateModRoot(repoRoot)
	if err != nil {
		return err
	}
	cliRoot := filepath.Join(modRoot, "cli")
	if _, err := os.Stat(cliRoot); err != nil {
		return fmt.Errorf("cli root missing: %s", cliRoot)
	}

	binPath := filepath.Join(repoRoot, "bin", "mod-v1")
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
	fmt.Printf("built mod v1 binary: %s\n", binPath)
	return nil
}
