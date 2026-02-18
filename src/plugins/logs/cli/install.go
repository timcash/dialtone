package cli

import (
	"dialtone/cli/src/core/config"
	core_install "dialtone/cli/src/core/install"
	"fmt"
	"os"
	"path/filepath"
)

var installRequirements = []core_install.Requirement{
	{Tool: core_install.ToolGo, Version: core_install.GoVersion},
	{Tool: core_install.ToolBun, Version: core_install.BunVersion},
}

func RunInstall(versionDir string) error {
	fmt.Printf(">> [LOGS] Install: %s\n", versionDir)

	if err := core_install.EnsureRequirements(installRequirements); err != nil {
		return err
	}

	_ = config.GetDialtoneEnv() // ensure env is loadable

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
