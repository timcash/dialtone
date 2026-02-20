package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func Install() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	repoRoot := cwd
	if filepath.Base(cwd) == "src" {
		repoRoot = filepath.Dir(cwd)
	}
	uiDir := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v1", "ui")

	if _, err := os.Stat(filepath.Join(uiDir, "package.json")); err != nil {
		return fmt.Errorf("missing src_v1 ui package.json: %w", err)
	}

	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "install", "--force")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
