package ops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

func UIRun(port int) error {
	paths, err := resolveRobotPathsPreset()
	if err != nil {
		return err
	}
	repoRoot := paths.Runtime.RepoRoot

	if port == 0 {
		port = 3000
	}
	uiDir := paths.Preset.UI

	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "bun", "src_v1", "exec", "--cwd", uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
