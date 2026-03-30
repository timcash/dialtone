package logsv1

import (
	"fmt"
	"os"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func RunBuild(versionDir string) error {
	logs.Info("logs %s build: start", versionDir)
	if err := RunGoBuild(versionDir); err != nil {
		return err
	}

	paths, err := resolveLogsPaths(versionDir)
	if err != nil {
		return err
	}
	uiDir := paths.Preset.UI

	if _, err := os.Stat(uiDir); os.IsNotExist(err) {
		return fmt.Errorf("UI directory not found: %s", uiDir)
	}

	logs.Info("logs %s build: installing UI dependencies in %s", versionDir, uiDir)
	installCmd := runBun(paths.Runtime.RepoRoot, uiDir, "install", "--force")
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("UI install failed: %v", err)
	}

	logs.Info("logs %s build: building UI in %s", versionDir, uiDir)
	cmd := runBun(paths.Runtime.RepoRoot, uiDir, "run", "build")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("UI build failed: %v", err)
	}

	logs.Info("logs %s build: complete", versionDir)
	return nil
}
