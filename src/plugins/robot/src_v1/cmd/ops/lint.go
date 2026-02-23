package ops

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Lint() error {
	paths, err := resolveRobotPathsPreset()
	if err != nil {
		return err
	}
	repoRoot := paths.Runtime.RepoRoot
	uiDir := paths.Preset.UI

	uiLint := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "bun", "src_v1", "exec", "--cwd", uiDir, "run", "lint")
	uiLint.Dir = repoRoot
	uiLint.Stdout = os.Stdout
	uiLint.Stderr = os.Stderr
	return uiLint.Run()
}
