package test

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Run03GoBuild(repoRoot string) error {
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "robot", "go-build", "src_v1")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
