package cli

import (
	"dialtone/cli/src/core/install"
	"fmt"
	"os"
	"path/filepath"
)

var installRequirements = []install.Requirement{
	{Tool: install.ToolGo, Version: install.GoVersion},
	{Tool: install.ToolBun, Version: install.BunVersion},
}

func RunInstall(versionDir string) error {
	if err := install.EnsureRequirements(installRequirements); err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "cloudflare", versionDir, "ui")

	if _, err := os.Stat(filepath.Join(uiDir, "package.json")); err != nil {
		return fmt.Errorf("ui package.json not found for %s: %w", versionDir, err)
	}

	fmt.Printf(">> [CLOUDFLARE] Install: %s\n", versionDir)
	fmt.Println("   [CLOUDFLARE] Running bun install...")
	cmd := runBun(cwd, uiDir, "install", "--force")
	return cmd.Run()
}
