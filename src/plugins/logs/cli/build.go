package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

func RunBuild(versionDir string) error {
	fmt.Printf(">> [LOGS] Build: START for %s\n", versionDir)

	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "logs", versionDir, "ui")

	if _, err := os.Stat(uiDir); os.IsNotExist(err) {
		return fmt.Errorf("UI directory not found: %s", uiDir)
	}

	fmt.Printf(">> [LOGS] Installing UI dependencies in %s...\n", uiDir)
	installCmd := runBun(cwd, uiDir, "install", "--force")
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("UI install failed: %v", err)
	}

	fmt.Printf(">> [LOGS] Building UI in %s...\n", uiDir)
	cmd := runBun(cwd, uiDir, "run", "build")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("UI build failed: %v", err)
	}

	fmt.Printf(">> [LOGS] Build: COMPLETE for %s\n", versionDir)
	return nil
}
