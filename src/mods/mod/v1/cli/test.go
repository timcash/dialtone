package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func runTest(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("mod test does not accept positional arguments")
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

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = cliRoot
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
