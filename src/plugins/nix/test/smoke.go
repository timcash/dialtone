package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dialtone/cli/src/core/browser"
	"dialtone/cli/src/dialtest"
	chrome_app "dialtone/cli/src/plugins/chrome/app"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type smokeEntry struct {
	name       string
	conditions string
	status     string
	errorMsg   string
	stackTrace string
	screenshot string
	logs       []string
}

func RunSmoke(versionDir string, timeoutSec int) error {
	fmt.Printf(">> [NIX] Smoke: START for %s\n", versionDir)
	
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "nix", versionDir)
	smokeFile := filepath.Join(pluginDir, "SMOKE.md")
	port := 8080

	browser.CleanupPort(port)
	cmd := exec.Command("go", "run", "cmd/main.go")
	cmd.Dir = pluginDir
	logFile, _ := os.Create(filepath.Join(pluginDir, "smoke_server.log"))
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	
	if err := cmd.Start(); err != nil {
		fmt.Printf("   [ERROR] Failed to start nix plugin: %v\n", err)
		return err
	}
	defer cmd.Process.Kill()

	if err := waitForPort(port, 15*time.Second); err != nil {
		fmt.Printf("   [ERROR] Host node timeout: %v\n", err)
		return err
	}

	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	fmt.Printf(">> [NIX] Plugin started. Access UI at: %s\n", url)

	wsURL, isNew, err := resolveChrome(0, true)
	if err != nil {
		fmt.Printf("   [ERROR] Chrome resolution failed: %v\n", err)
		return err
	}
	fmt.Printf(">> [NIX] Chrome WebSocket: %s\n", wsURL)

	defer func() {
		if isNew {
			exec.Command("./dialtone.sh", "chrome", "kill", "all").Run()
		}
	}()

	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	var mu sync.Mutex
	var currentLogs []string
	
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			msg := formatConsoleArgs(ev.Args)
			mu.Lock()
			currentLogs = append(currentLogs, fmt.Sprintf("[%s] %s", ev.Type, msg))
			mu.Unlock()
			fmt.Printf("   [BROWSER] [%s] %s\n", ev.Type, msg)
		}
	})

	testResults := []smokeEntry{}
	os.WriteFile(smokeFile, []byte("# Nix Robust Smoke Test Report\n\n**Started:** "+time.Now().Format(time.RFC1123)+"\n"), 0644)

	runStep := func(name string, actions chromedp.Action) error {
		fmt.Printf(">> [NIX] Step: %s\n", name)
		
		if err := chromedp.Run(ctx, actions); err != nil {
			fmt.Printf("   [ERROR] Action failed: %v\n", err)
			return err
		}

		var buf []byte
		_ = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			b, err := page.CaptureScreenshot().Do(ctx)
			buf = b
			return err
		}))
		
		shotName := fmt.Sprintf("smoke_step_%d.png", len(testResults))
		if len(buf) > 0 {
			os.WriteFile(filepath.Join(pluginDir, shotName), buf, 0644)
		}

		f, _ := os.OpenFile(smokeFile, os.O_APPEND|os.O_WRONLY, 0644)
		defer f.Close()
		fmt.Fprintf(f, "\n### %s\n\n![%s](%s)\n\n---", name, name, shotName)
		
		testResults = append(testResults, smokeEntry{name: name})
		return nil
	}

	// Initial Navigation
	if err := chromedp.Run(ctx, 
		chromedp.EmulateViewport(1280, 800),
		chromedp.Navigate(fmt.Sprintf("http://127.0.0.1:%d", port)),
		dialtest.WaitForAriaLabel("Nix Hero Title"),
	); err != nil { 
		fmt.Printf("   [ERROR] Initial navigation failed: %v\n", err)
		return err 
	}

	if err := runStep("1. Hero Section Validation", dialtest.WaitForAriaLabel("Nix Hero Title")); err != nil { return err }
	if err := runStep("2. Documentation Section Validation", dialtest.NavigateToSection("nix-docs", "Nix Documentation Title")); err != nil { return err }
	if err := runStep("3. Verify Header Hidden", dialtest.AssertElementHidden(".header-title")); err != nil { return err }
	if err := runStep("4. Nix Table Rendering", dialtest.NavigateToSection("nix-table", "Nix Process Table")); err != nil { return err }
	
	if err := runStep("5. Spawn Nix Nodes", chromedp.Tasks{
		chromedp.Click(`#start-node`, chromedp.ByQuery),
		chromedp.WaitVisible("#proc-1", chromedp.ByQuery),
		chromedp.Click(`#start-node`, chromedp.ByQuery),
		chromedp.WaitVisible("#proc-2", chromedp.ByQuery),
	}); err != nil { return err }

	if err := runStep("6. Selective Termination", chromedp.Tasks{
		// More robust: click and wait for state change with retries
		chromedp.ActionFunc(func(ctx context.Context) error {
			start := time.Now()
			for time.Since(start) < 15*time.Second {
				// Try to click via JS directly (more robust for dynamic elements)
				_ = chromedp.Run(ctx, chromedp.Evaluate(`
					(function() {
						const btn = document.querySelector("#proc-1 .stop-btn");
						if (btn) {
							btn.click();
							return true;
						}
						return false;
					})()
				`, nil))

				// Check if stopped
				var isStopped bool
				_ = chromedp.Run(ctx, chromedp.Evaluate(`
					(function() {
						const badge = document.querySelector("#proc-1 .status-badge");
						return badge && badge.getAttribute("data-status-text") === "stopped";
					})()
				`, &isStopped))

				if isStopped {
					return nil
				}
				time.Sleep(1 * time.Second)
			}
			return fmt.Errorf("timeout waiting for proc-1 to stop")
		}),
	}); err != nil { return err }

	if err := runStep("7. Verify Persistence", dialtest.WaitForAriaLabel("Nix Process Table")); err != nil { return err }

	fmt.Printf(">> [NIX] Smoke: COMPLETE. Report at %s\n", smokeFile)
	return nil
}

func waitForPort(port int, timeout time.Duration) error {
	start := time.Now()
	for time.Since(start) < timeout {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout")
}

func resolveChrome(requestedPort int, headless bool) (string, bool, error) {
	procs, err := chrome_app.ListResources(true)
	if err == nil {
		for _, p := range procs {
			if p.DebugPort > 0 {
				resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", p.DebugPort))
				if err == nil {
					var data struct {
						WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
					}
					json.NewDecoder(resp.Body).Decode(&data)
					resp.Body.Close()
					if data.WebSocketDebuggerURL != "" {
						return data.WebSocketDebuggerURL, false, nil
					}
				}
			}
		}
	}
	res, err := chrome_app.LaunchChrome(requestedPort, true, headless, "")
	if err != nil {
		return "", false, err
	}
	return res.WebsocketURL, true, nil
}

func formatConsoleArgs(args []*runtime.RemoteObject) string {
	var parts []string
	for _, arg := range args {
		if arg == nil {
			continue
		}
		if len(arg.Value) > 0 {
			var v interface{}
			if err := json.Unmarshal(arg.Value, &v); err == nil {
				b, _ := json.Marshal(v)
				parts = append(parts, string(b))
			} else {
				parts = append(parts, string(arg.Value))
			}
		} else if arg.Description != "" {
			parts = append(parts, arg.Description)
		}
	}
	return strings.Join(parts, " ")
}