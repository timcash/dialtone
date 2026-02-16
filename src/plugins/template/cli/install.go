package cli

import (
	"dialtone/cli/src/core/install"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var installRequirements = []install.Requirement{
	{Tool: install.ToolGo, Version: install.GoVersion},
	{Tool: install.ToolBun, Version: install.BunVersion},
}

func runTemplateInstall(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Install: %s\n", versionDir)
	return runInstall(versionDir)
}

func runInstall(versionDir string) error {
	if err := install.EnsureRequirements(installRequirements); err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "template", versionDir, "ui")

	// If versionDir is already an absolute path or contains src/plugins, use it directly
	if filepath.IsAbs(versionDir) {
		uiDir = filepath.Join(versionDir, "ui")
	} else if strings.HasPrefix(versionDir, "src/plugins") {
		uiDir = filepath.Join(cwd, versionDir, "ui")
	}

	if _, err := os.Stat(filepath.Join(uiDir, "package.json")); err != nil {
		return fmt.Errorf("ui package.json not found for %s: %w", versionDir, err)
	}

	fmt.Println("   [TEMPLATE] Running bun install...")
	cmd := runBun(cwd, uiDir, "install", "--force")
	return cmd.Run()
}
