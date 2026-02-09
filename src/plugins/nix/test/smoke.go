package test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"dialtone/cli/src/core/browser"
	chrome_app "dialtone/cli/src/plugins/chrome/app"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"encoding/json"
	"net/http"
	"io"
)

func RunSmoke(versionDir string) error {
	fmt.Printf(">> [NIX] Smoke: start for %s\n", versionDir)
	
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "nix", versionDir)
	smokeFile := filepath.Join(pluginDir, "SMOKE.md")
	port := 8080

	// 1. Start the Nix Plugin server (Host Node)
	fmt.Printf(">> [NIX] Smoke: starting host node in %s...\n", versionDir)
	cmd := exec.Command("go", "run", "cmd/main.go")
	cmd.Dir = pluginDir
	logFile, _ := os.Create(filepath.Join(pluginDir, "smoke_server.log"))
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start nix plugin: %v", err)
	}
	defer cmd.Process.Kill()

	// 2. Wait for port 8080
	if err := waitForPort(port, 15*time.Second); err != nil {
		return fmt.Errorf("host node not ready on %d: %v", port, err)
	}

	// 3. Connect to Chrome
	wsURL, err := getChromeWS()
	if err != nil {
		return fmt.Errorf("failed to get chrome websocket: %v", err)
	}

	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	defer cancel()
	// Add a global timeout for the smoke test operations
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	// Console logs collection
	var consoleLogs []string
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			for _, arg := range ev.Args {
				var val interface{}
				json.Unmarshal(arg.Value, &val)
				consoleLogs = append(consoleLogs, fmt.Sprintf("[%s] %v", ev.Type, val))
			}
		}
	})

	// 4. Test UI Flow
	fmt.Println(">> [NIX] Smoke: Testing UI operations...")
	var buf1, buf2 []byte
	var procID string
	err = chromedp.Run(ctx,
		chromedp.Navigate(fmt.Sprintf("http://127.0.0.1:%d", port)),
		chromedp.WaitVisible("#status", chromedp.ByQuery),
		chromedp.Click("#error-ping", chromedp.ByQuery),
		chromedp.Click("#start-proc", chromedp.ByQuery),
		chromedp.WaitVisible(".proc-container", chromedp.ByQuery),
		chromedp.Sleep(4*time.Second), // Wait for logs to appear
		chromedp.Screenshot("#app", &buf1, chromedp.NodeVisible),
		chromedp.AttributeValue(".proc-item", "id", &procID, nil, chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Errorf("chromedp failed during operations: %v", err)
	}
	os.WriteFile(filepath.Join(pluginDir, "smoke_step1_active.png"), buf1, 0644)

	// 5. Stop the sub-process
	fmt.Println(">> [NIX] Smoke: Testing 'Stop Sub-Process'...")
	err = chromedp.Run(ctx,
		chromedp.Click(fmt.Sprintf("#stop-%s", procID), chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
		chromedp.Screenshot("#app", &buf2, chromedp.NodeVisible),
	)
	if err != nil {
		return fmt.Errorf("chromedp failed during stop: %v", err)
	}
	os.WriteFile(filepath.Join(pluginDir, "smoke_step2_stopped.png"), buf2, 0644)

	// 6. Update SMOKE.md
	entry := fmt.Sprintf("# Nix Smoke Test Report (%s)\n\n", time.Now().Format("15:04:05"))
	entry += "## Results: ✅ PASSED\n\n"
	entry += fmt.Sprintf("- **Host Node**: Running on port `%d`\n", port)
	entry += fmt.Sprintf("- **Version**: `%s`\n", versionDir)
	
	entry += "\n### Visual Proof\n"
	entry += "| Step 1: Sub-process Started | Step 2: Sub-process Stopped |\n"
	entry += "|---|---|\n"
	entry += "| ![Step 1](smoke_step1_active.png) | ![Step 2](smoke_step2_stopped.png) |\n\n"

	entry += "### Browser Console Output\n```\n"
	for _, l := range consoleLogs {
		entry += l + "\n"
	}
	entry += "```\n\n"

	entry += "### Host Server Logs\n```\n"
	logs, _ := os.ReadFile(filepath.Join(pluginDir, "smoke_server.log"))
	entry += string(logs)
	entry += "\n```\n\n---\n"
	
	os.WriteFile(smokeFile, []byte(entry), 0644)

	fmt.Println(">> [NIX] Smoke: complete. Report generated in SMOKE.md")
	return nil
}

func waitForPort(port int, timeout time.Duration) error {
	start := time.Now()
	for time.Since(start) < timeout {
		if browser.IsPortOpen(port) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for port %d", port)
}

func getChromeWS() (string, error) {
	procs, err := chrome_app.ListResources(true)
	if err == nil {
		for _, p := range procs {
			if p.DebugPort > 0 {
				url, err := fetchWSURL(p.DebugPort)
				if err == nil {
					return url, nil
				}
			}
		}
	}
	fmt.Println(">> [NIX] Smoke: launching new chrome...")
	res, err := chrome_app.LaunchChrome(0, false, true, "")
	if err != nil {
		return "", err
	}
	return res.WebsocketURL, nil
}

func fetchWSURL(port int) (string, error) {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var data struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	json.Unmarshal(body, &data)
	return data.WebSocketDebuggerURL, nil
}