package cli

import (
	"fmt"
)

func RunLint(versionDir string) error {
	fmt.Printf(">> [LOGS] Lint: %s\n", versionDir)
	paths, err := resolveLogsPaths(versionDir)
	if err != nil {
		return err
	}

	uiDir := paths.Preset.UI
	cmd := runBun(paths.Runtime.RepoRoot, uiDir, "run", "lint")
	return cmd.Run()
}
