package test

import (
	"os"
	"os/exec"
	"path/filepath"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

// RunAll keeps compatibility with older callers while delegating to src_v1 tests.
func RunAll() error {
	logs.Info("Running bun plugin suite via src_v1 test runner...")

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	cmd := exec.Command("go", "run", "./plugins/bun/src_v1/test/cmd/main.go")
	cmd.Dir = filepath.Join(repoRoot, "src")
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
			return "", logs.Errorf("repo root not found")
		}
		cwd = parent
	}
}
