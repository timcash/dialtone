package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"dialtone/cli/src/libs/dialtest"
	"github.com/chromedp/chromedp"
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
	pluginDir := filepath.Join(cwd, "src", "plugins", "wsl", versionDir)
	smokeDir := filepath.Join(pluginDir, "smoke")

	runner, err := dialtest.NewSmokeRunner(dialtest.SmokeOptions{
		Name:           "WSL",
		VersionDir:     versionDir,
		Port:           8080,
		SmokeDir:       smokeDir,
		TotalTimeout:   180 * time.Second,
		StepTimeout:    30 * time.Second,
		CommandStall:   30 * time.Second,
		PanicOnTimeout: true,
	})
	if err != nil {
		return err
	}
	defer runner.Finalize()

	serverCmd, err := runner.PrepareGoPluginSmoke(cwd, "wsl", nil)
	if err != nil {
		return err
	}
	defer serverCmd.Process.Kill()

	runner.Step("Home Section Validation", dialtest.WaitForAriaLabel("WSL Hero Section"))
	runner.Step("Documentation Section Validation", dialtest.NavigateToSection("docs", "WSL Documentation Section"))
	runner.Step("Table Section Validation", dialtest.NavigateToSection("table", "WSL Spreadsheet Section"))
	runner.Step("Verify Header Hidden on Table", dialtest.AssertElementHidden(".header-title"))

	testNode := "smoke-v2-node"
	runner.Step("Spawn WSL Node", chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(fmt.Sprintf(`window.prompt = () => "%s";`, testNode), nil).Do(ctx)
		}),
		chromedp.WaitVisible(`button[aria-label="Spawn WSL Node"]`, chromedp.ByQuery),
		chromedp.Click(`button[aria-label="Spawn WSL Node"]`, chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			start := time.Now()
			for time.Since(start) < 45*time.Second {
				var isRunning bool
				_ = chromedp.Evaluate(fmt.Sprintf(`document.body.innerText.includes("%s") && document.body.innerText.includes("RUNNING")`, testNode), &isRunning).Do(ctx)
				if isRunning {
					return nil
				}
				time.Sleep(2 * time.Second)
			}
			return fmt.Errorf("timeout waiting for %s to reach RUNNING state", testNode)
		}),
	})

	runner.Step("Stop Node", chromedp.Tasks{
		chromedp.WaitVisible(fmt.Sprintf(`button[aria-label="Stop Node %s"]`, testNode), chromedp.ByQuery),
		chromedp.Click(fmt.Sprintf(`button[aria-label="Stop Node %s"]`, testNode), chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			start := time.Now()
			for time.Since(start) < 20*time.Second {
				var isStopped bool
				_ = chromedp.Evaluate(fmt.Sprintf(`document.body.innerText.includes("STOPPED") && document.body.innerText.includes("%s")`, testNode), &isStopped).Do(ctx)
				if isStopped {
					return nil
				}
				time.Sleep(1 * time.Second)
			}
			return fmt.Errorf("timeout waiting for %s to stop", testNode)
		}),
	})

	runner.Step("Delete Node", chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(`window.confirm = () => true;`, nil).Do(ctx)
		}),
		chromedp.WaitVisible(fmt.Sprintf(`button[aria-label="Delete Node %s"]`, testNode), chromedp.ByQuery),
		chromedp.Click(fmt.Sprintf(`button[aria-label="Delete Node %s"]`, testNode), chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			start := time.Now()
			for time.Since(start) < 20*time.Second {
				var found bool
				_ = chromedp.Evaluate(fmt.Sprintf(`document.body.innerText.includes("%s")`, testNode), &found).Do(ctx)
				if !found {
					return nil
				}
				time.Sleep(1 * time.Second)
			}
			return fmt.Errorf("timeout waiting for %s to be deleted", testNode)
		}),
	})

	runner.Step("Return Home", dialtest.NavigateToSection("home", "Home Section"))

	if err := runner.AssertSectionLifecycle([]string{"home", "docs", "table"}); err != nil {
		return err
	}

	return nil
}
