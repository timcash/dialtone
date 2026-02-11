package main

import (
	"fmt"
	"os"
	"path/filepath"

	"dialtone/cli/src/libs/dialtest"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: smoke <versionDir>")
		os.Exit(1)
	}
	versionDir := os.Args[1]
	if err := Run(versionDir); err != nil {
		fmt.Printf("Smoke test failed: %v\n", err)
		os.Exit(1)
	}
}

func Run(versionDir string) error {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "template", versionDir)
	smokeDir := filepath.Join(pluginDir, "smoke")

	runner, err := dialtest.NewSmokeRunner(dialtest.SmokeOptions{
		Name:       "Template",
		VersionDir: versionDir,
		Port:       8080,
		SmokeDir:   smokeDir,
	})
	if err != nil {
		return err
	}
	defer runner.Finalize()

	serverCmd, err := runner.PrepareGoPluginSmoke(cwd, "template", nil)
	if err != nil {
		return err
	}
	defer serverCmd.Process.Kill()

	runner.Step("Hero Section Validation", dialtest.WaitForAriaLabel("Home Section"))
	runner.Step("Documentation Section Validation", dialtest.NavigateToSection("docs", "Docs Section"))
	runner.Step("Table Section Validation", dialtest.NavigateToSection("table", "Table Section"))
	runner.Step("Verify Header Hidden on Table", dialtest.AssertElementHidden(".header-title"))
	runner.Step("Settings Section Validation", dialtest.NavigateToSection("settings", "Settings Section"))
	runner.Step("Return Home", dialtest.NavigateToSection("home", "Home Section"))

	return nil
}
