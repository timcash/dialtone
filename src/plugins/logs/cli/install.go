package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

func RunInstall(versionDir string) error {
	fmt.Printf(">> [LOGS] Install: %s\n", versionDir)

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	uiDir := filepath.Join(cwd, "src", "plugins", "logs", versionDir, "ui")
	if _, err := os.Stat(filepath.Join(uiDir, "package.json")); err != nil {
		return fmt.Errorf("ui package.json not found for %s: %w", versionDir, err)
	}

	cmd := runBun(cwd, uiDir, "install", "--force")
	return cmd.Run()
}
