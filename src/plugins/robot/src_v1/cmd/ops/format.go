package ops

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Format() error {
	paths, err := resolveRobotPathsPreset()
	if err != nil {
		return err
	}
	repoRoot := paths.Runtime.RepoRoot
	uiDir := paths.Preset.UI

	uiFmt := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "bun", "src_v1", "exec", "--cwd", uiDir, "run", "format")
	uiFmt.Dir = repoRoot
	uiFmt.Stdout = os.Stdout
	uiFmt.Stderr = os.Stderr
	return uiFmt.Run()
}
