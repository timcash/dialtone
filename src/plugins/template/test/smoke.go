package test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dialtone/cli/src/core/browser"
	"dialtone/cli/src/dialtest"
	chrome_app "dialtone/cli/src/plugins/chrome/app"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func RunSmoke(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Smoke: %s\n", versionDir)
	
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "template", versionDir)
	smokeFile := filepath.Join(pluginDir, "SMOKE.md")
	port := 8080

	browser.CleanupPort(port)
	cmd := exec.Command("go", "run", "cmd/main.go")
	cmd.Dir = pluginDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Process.Kill()

	if err := waitForPort(port, 10*time.Second); err != nil {
		return err
	}

	url := fmt.Sprintf("http://localhost:%d", port)
	fmt.Printf(">> [TEMPLATE] Plugin started. Access UI at: %s\n", url)

	wsURL, isNew, err := resolveChrome(0, true)
	if err != nil {
		return err
	}
	defer func() {
		if isNew {
			exec.Command("./dialtone.sh", "chrome", "kill", "all").Run()
		}
	}()

	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	os.WriteFile(smokeFile, []byte("# Template Smoke Test Report\n\n**Started:** "+time.Now().Format(time.RFC1123)+"\n"), 0644)

	runStep := func(name string, actions chromedp.Action) error {
		fmt.Printf(">> [TEMPLATE] Step: %s\n", name)
		
		if err := chromedp.Run(ctx, actions); err != nil {
			return err
		}

		var buf []byte
		_ = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			b, err := page.CaptureScreenshot().Do(ctx)
			buf = b
			return err
		}))

		shotName := fmt.Sprintf("smoke_%s.png", strings.ToLower(strings.ReplaceAll(name, " ", "_")))
		if len(buf) > 0 {
			os.WriteFile(filepath.Join(pluginDir, shotName), buf, 0644)
		}

		f, _ := os.OpenFile(smokeFile, os.O_APPEND|os.O_WRONLY, 0644)
		defer f.Close()
		fmt.Fprintf(f, "\n### %s\n\n![%s](%s)\n\n---", name, name, shotName)
		
		return nil
	}

	if err := chromedp.Run(ctx, 
		chromedp.EmulateViewport(1280, 800),
		chromedp.Navigate(fmt.Sprintf("http://localhost:%d", port)),
	); err != nil { return err }

	if err := runStep("Initial Home", dialtest.WaitForAriaLabel("Home Section")); err != nil { return err }
	if err := runStep("Settings Section", dialtest.NavigateToSection("settings", "Settings Section")); err != nil { return err }
	if err := runStep("Verify Header and Menu Hidden", chromedp.Tasks{
		dialtest.AssertElementHidden(".main-header"),
		dialtest.AssertElementHidden("#global-menu"),
	}); err != nil { return err }
	if err := runStep("Return Home", dialtest.NavigateToSection("home", "Home Section")); err != nil { return err }

	fmt.Printf(">> [TEMPLATE] Smoke: COMPLETE. Report at %s\n", smokeFile)
	return nil
}

func waitForPort(port int, timeout time.Duration) error {
	start := time.Now()
	for time.Since(start) < timeout {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for port %d", port)
}

func resolveChrome(requestedPort int, headless bool) (string, bool, error) {
	res, err := chrome_app.LaunchChrome(requestedPort, true, headless, "")
	if err != nil {
		return "", false, err
	}
	return res.WebsocketURL, true, nil
}
