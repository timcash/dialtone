package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"dialtone/dev/libs/dialtest"
	"github.com/chromedp/chromedp"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: smoke <versionDir> [timeoutSec]")
		os.Exit(1)
	}
	versionDir := os.Args[1]
	timeoutSec := 45
	if len(os.Args) > 2 {
		if parsed, err := strconv.Atoi(os.Args[2]); err == nil && parsed > 0 {
			timeoutSec = parsed
		}
	}

	if err := Run(versionDir, timeoutSec); err != nil {
		fmt.Printf("Smoke test failed: %v\n", err)
		os.Exit(1)
	}
}

func Run(versionDir string, timeoutSec int) error {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "dag", versionDir)
	smokeDir := filepath.Join(pluginDir, "smoke")

	runner, err := dialtest.NewSmokeRunner(dialtest.SmokeOptions{
		Name:           "Dag",
		VersionDir:     versionDir,
		Port:           8080,
		SmokeDir:       smokeDir,
		TotalTimeout:   time.Duration(timeoutSec) * time.Second,
		StepTimeout:    7 * time.Second,
		CommandStall:   10 * time.Second,
		PanicOnTimeout: true,
	})
	if err != nil {
		return err
	}
	defer runner.Finalize()

	serverCmd, err := runner.PrepareGoPluginSmoke(cwd, "dag", nil)
	if err != nil {
		return err
	}
	defer serverCmd.Process.Kill()

	runner.Step("Hero Section Validation", chromedp.Tasks{
		dialtest.WaitForAriaLabel("DAG Hero Title"),
		dialtest.WaitForAriaLabel("DAG Hero Canvas"),
		chromedp.WaitVisible(".header-title", chromedp.ByQuery),
		chromedp.WaitVisible(".top-right-controls", chromedp.ByQuery),
		chromedp.WaitVisible(".main-nav", chromedp.ByQuery),
	})
	if err := runner.AssertLastStepLogsContains(
		"[SectionManager] 📦 LOADING #dag-hero",
		"[SectionManager] ✅ LOADED #dag-hero",
		"[SectionManager] ✨ START #dag-hero",
		"[SectionManager] 🚀 RESUME #dag-hero",
	); err != nil {
		return err
	}

	runner.Step("Docs Section Validation", chromedp.Tasks{
		dialtest.NavigateToSection("dag-docs", "DAG Docs Title"),
		dialtest.WaitForAriaLabel("DAG Docs Commands"),
		dialtest.AssertElementHidden(".header-title"),
		dialtest.AssertElementHidden(".top-right-controls"),
		dialtest.AssertElementHidden(".main-nav"),
	})
	if err := runner.AssertLastStepLogsContains(
		"[SectionManager] 🧭 NAVIGATING TO #dag-docs",
		"[SectionManager] 🧭 NAVIGATE TO #dag-docs",
		"[SectionManager] 🚀 RESUME #dag-docs",
	); err != nil {
		return err
	}

	runner.Step("Layer Section Validation", chromedp.Tasks{
		dialtest.NavigateToSection("dag-layer-nest", "DAG Layer Canvas"),
		dialtest.AssertElementHidden(".header-title"),
		dialtest.AssertElementHidden(".top-right-controls"),
		dialtest.AssertElementHidden(".main-nav"),
	})
	if err := runner.AssertLastStepLogsContains(
		"[SectionManager] 🧭 NAVIGATING TO #dag-layer-nest",
		"[SectionManager] 🧭 NAVIGATE TO #dag-layer-nest",
		"[SectionManager] 🚀 RESUME #dag-layer-nest",
	); err != nil {
		return err
	}

	runner.Step("Return Hero", chromedp.Tasks{
		dialtest.NavigateToSection("dag-hero", "DAG Hero Title"),
		chromedp.WaitVisible(".header-title", chromedp.ByQuery),
		chromedp.WaitVisible(".top-right-controls", chromedp.ByQuery),
		chromedp.WaitVisible(".main-nav", chromedp.ByQuery),
	})
	if err := runner.AssertLastStepLogsContains(
		"[SectionManager] 🧭 NAVIGATING TO #dag-hero",
		"[SectionManager] 🧭 NAVIGATE TO #dag-hero",
		"[SectionManager] 🚀 RESUME #dag-hero",
		"[SectionManager] 🧭 NAVIGATE AWAY #dag-layer-nest",
	); err != nil {
		return err
	}

	if err := runner.AssertSectionLifecycle([]string{"dag-hero", "dag-docs", "dag-layer-nest"}); err != nil {
		return err
	}

	return nil
}
