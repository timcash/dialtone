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
	if err := runner.AssertLastStepLogsContains(
		"[SectionManager] 📦 LOADING #home",
		"[SectionManager] ✅ LOADED #home",
		"[SectionManager] ✨ START #home",
		"[SectionManager] 🚀 RESUME #home",
	); err != nil {
		return err
	}

	runner.Step("Documentation Section Validation", dialtest.NavigateToSection("docs", "Docs Section"))
	if err := runner.AssertLastStepLogsContains(
		"[SectionManager] 🧭 NAVIGATING TO #docs",
		"[SectionManager] 🧭 NAVIGATE TO #docs",
		"[SectionManager] 🚀 RESUME #docs",
		"[SectionManager] 💤 PAUSE #home",
		"[SectionManager] 🧭 NAVIGATE AWAY #home",
	); err != nil {
		return err
	}

	runner.Step("Table Section Validation", dialtest.NavigateToSection("table", "Table Section"))
	if err := runner.AssertLastStepLogsContains(
		"[SectionManager] 🧭 NAVIGATING TO #table",
		"[SectionManager] 🧭 NAVIGATE TO #table",
		"[SectionManager] 🚀 RESUME #table",
		"[SectionManager] 💤 PAUSE #docs",
		"[SectionManager] 🧭 NAVIGATE AWAY #docs",
	); err != nil {
		return err
	}

	runner.Step("Verify Header Hidden on Table", dialtest.AssertElementHidden(".header-title"))

	runner.Step("Settings Section Validation", dialtest.NavigateToSection("settings", "Settings Section"))
	if err := runner.AssertLastStepLogsContains(
		"[SectionManager] 🧭 NAVIGATING TO #settings",
		"[SectionManager] 🧭 NAVIGATE TO #settings",
		"[SectionManager] 🚀 RESUME #settings",
		"[SectionManager] 💤 PAUSE #table",
		"[SectionManager] 🧭 NAVIGATE AWAY #table",
	); err != nil {
		return err
	}

	runner.Step("Return Home", dialtest.NavigateToSection("home", "Home Section"))
	if err := runner.AssertLastStepLogsContains(
		"[SectionManager] 🧭 NAVIGATING TO #home",
		"[SectionManager] 🧭 NAVIGATE TO #home",
		"[SectionManager] 🚀 RESUME #home",
		"[SectionManager] 🧭 NAVIGATE AWAY #settings",
	); err != nil {
		return err
	}

	if err := runner.AssertSectionLifecycle([]string{"home", "docs", "table", "settings"}); err != nil {
		return err
	}

	return nil
}
