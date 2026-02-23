package ops

import (
	"os"
	"os/exec"
	"path/filepath"
)

func GoBuild() error {
	repoRoot, _, err := resolveRobotPaths()
	if err != nil {
		return err
	}

	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "go", "src_v1", "exec", "build", "./plugins/robot/src_v1/...")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
