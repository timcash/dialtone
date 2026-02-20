package ops

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Format() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	repoRoot := cwd
	if filepath.Base(cwd) == "src" {
		repoRoot = filepath.Dir(cwd)
	}
	uiDir := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v1", "ui")

	uiFmt := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "format")
	uiFmt.Dir = repoRoot
	uiFmt.Stdout = os.Stdout
	uiFmt.Stderr = os.Stderr
	return uiFmt.Run()
}
