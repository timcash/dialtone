package go_plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func RunGo(args ...string) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), append([]string{"go", "exec"}, args...)...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("repo root not found from %s", cwd)
		}
		cwd = parent
	}
}
