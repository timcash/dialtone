package test

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Run04UILint(repoRoot string) error {
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "robot", "lint", "src_v1")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
