package src_v3

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	core_install "dialtone/cli/src/core/install"
)

var installRequirements = []core_install.Requirement{
	{Tool: core_install.ToolGo, Version: core_install.GoVersion},
	{Tool: core_install.ToolBun, Version: core_install.BunVersion},
}

func Install() error {
	if err := core_install.EnsureRequirements(installRequirements); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	uiDir := filepath.Join(cwd, "src", "plugins", "template", "src_v3", "ui")

	if _, err := os.Stat(filepath.Join(uiDir, "package.json")); err != nil {
		return fmt.Errorf("missing src_v3 ui package.json: %w", err)
	}

	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "install", "--force")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
