package ops

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Lint() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	repoRoot := cwd
	if filepath.Base(cwd) == "src" {
		repoRoot = filepath.Dir(cwd)
	}
	uiDir := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v1", "ui")

	uiLint := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "lint")
	uiLint.Dir = repoRoot
	uiLint.Stdout = os.Stdout
	uiLint.Stderr = os.Stderr
	return uiLint.Run()
}
