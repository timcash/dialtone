package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"dialtone/cli/src/core/browser"
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

	runner.RunPreflight(cwd, []struct{ Name, Cmd string; Args []string }{
		{"Install", "bun", []string{"install"}},
		{"Lint", "bun", []string{"run", "lint"}},
		{"Build", "bun", []string{"run", "build"}},
	})

	browser.CleanupPort(runner.Opts.Port)
	serverCmd := exec.Command("go", "run", "cmd/main.go")
	serverCmd.Dir = pluginDir
	if err := runner.StartServer(serverCmd); err != nil {
		return err
	}
	defer serverCmd.Process.Kill()

	if err := runner.SetupBrowser("http://127.0.0.1:8080"); err != nil {
		return err
	}

	runner.Step("Hero Section Validation", dialtest.WaitForAriaLabel("Home Section"))
	runner.Step("Documentation Section Validation", dialtest.NavigateToSection("docs", "Docs Section"))
	runner.Step("Table Section Validation", dialtest.NavigateToSection("table", "Table Section"))
	runner.Step("Verify Header Hidden on Table", dialtest.AssertElementHidden(".header-title"))
	runner.Step("Settings Section Validation", dialtest.NavigateToSection("settings", "Settings Section"))
	runner.Step("Return Home", dialtest.NavigateToSection("home", "Home Section"))

	return nil
}
