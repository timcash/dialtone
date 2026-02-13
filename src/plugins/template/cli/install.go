package cli

import (
	"fmt"
	"os"
	"path/filepath"

	template_v3 "dialtone/cli/src/plugins/template/src_v3/cmd/ops"
)

func runTemplateInstall(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Install: %s\n", versionDir)

	switch versionDir {
	case "src_v3":
		return runInstallV3()
	default:
		return runInstallLegacy(versionDir)
	}
}

func runInstallLegacy(versionDir string) error {
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "template", versionDir, "ui")

	if _, err := os.Stat(filepath.Join(uiDir, "package.json")); err != nil {
		return fmt.Errorf("ui package.json not found for %s: %w", versionDir, err)
	}

	fmt.Println("   [TEMPLATE] Running bun install...")
	cmd := runBun(cwd, uiDir, "install", "--force")
	return cmd.Run()
}

func runInstallV3() error {
	return template_v3.Install()
}
