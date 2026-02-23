package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

func RunInstall(versionDir string) error {
	fmt.Printf(">> [LOGS] Install: %s\n", versionDir)

	paths, err := resolveLogsPaths(versionDir)
	if err != nil {
		return err
	}
	uiDir := paths.Preset.UI
	if _, err := os.Stat(filepath.Join(uiDir, "package.json")); err != nil {
		return fmt.Errorf("ui package.json not found for %s: %w", versionDir, err)
	}

	cmd := runBun(paths.Runtime.RepoRoot, uiDir, "install", "--force")
	return cmd.Run()
}
