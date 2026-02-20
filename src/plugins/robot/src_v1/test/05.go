package test

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Run05UIFormat(repoRoot string) error {
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "robot", "format", "src_v1")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
