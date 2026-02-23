package test

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Run05UIFormat(repoRoot string) error {
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "robot", "src_v1", "format")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
