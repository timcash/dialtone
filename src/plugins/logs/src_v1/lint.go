package logsv1

import (
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func RunLint(versionDir string) error {
	logs.Info("logs %s lint", versionDir)
	paths, err := resolveLogsPaths(versionDir)
	if err != nil {
		return err
	}

	uiDir := paths.Preset.UI
	cmd := runBun(paths.Runtime.RepoRoot, uiDir, "run", "lint")
	return cmd.Run()
}
