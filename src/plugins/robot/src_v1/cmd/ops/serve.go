package ops

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Serve(repoRoot string) error {
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "go", "exec", "run", "plugins/robot/src_v1/cmd/server/main.go")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
