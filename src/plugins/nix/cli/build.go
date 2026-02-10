package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func RunBuild(versionDir string) error {
	fmt.Printf(">> [NIX] Build: START for %s\n", versionDir)

	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "nix", versionDir, "ui")

	if _, err := os.Stat(uiDir); os.IsNotExist(err) {
		return fmt.Errorf("UI directory not found: %s", uiDir)
	}

	fmt.Printf(">> [NIX] Building UI in %s...\n", uiDir)

	cmd := exec.Command("bun", "run", "build")
	cmd.Dir = uiDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("UI build failed: %v", err)
	}

	fmt.Printf(">> [NIX] Build: COMPLETE for %s\n", versionDir)
	return nil
}
