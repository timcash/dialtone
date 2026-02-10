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
	chrome_app "dialtone/cli/src/plugins/chrome/app"
	"github.com/chromedp/cdproto/page"
	cdruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func RunSmoke(versionDir string, timeoutSec int) error {
	fmt.Printf(">> [WSL] Smoke: START for %s\n", versionDir)
	
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "wsl", versionDir)
	smokeFile := filepath.Join(pluginDir, "SMOKE.md")
	port := 8080

	// 1. Initial Report Setup with Lint & Build verification
	reportHeader := "# WSL Robust Smoke Test Report\n\n"
	reportHeader += "**Started:** " + time.Now().Format(time.RFC1123) + "\n\n"
	reportHeader += "## Pre-test Verification\n\n"

	// LINT GO
	fmt.Println(">> [WSL] Smoke: Verifying Go standards...")
	lintGoCmd := exec.Command("go", "vet", "./src/plugins/wsl/...")
	if err := lintGoCmd.Run(); err != nil {
		reportHeader += "- [ ] **Go Vet:** FAILED\n"
	} else {
		reportHeader += "- [x] **Go Vet:** PASSED\n"
	}

	// LINT TS
	fmt.Println(">> [WSL] Smoke: Verifying TypeScript standards...")
	uiDir := filepath.Join(pluginDir, "ui")
	lintTsCmd := exec.Command("bun", "run", "lint")
	lintTsCmd.Dir = uiDir
	if err := lintTsCmd.Run(); err != nil {
		reportHeader += "- [ ] **TypeScript Lint:** FAILED\n"
	} else {
		reportHeader += "- [x] **TypeScript Lint:** PASSED\n"
	}

	// BUILD UI
	fmt.Println(">> [WSL] Smoke: Verifying UI build...")
	buildCmd := exec.Command("bun", "run", "build")
	buildCmd.Dir = uiDir
	if err := buildCmd.Run(); err != nil {
		reportHeader += "- [ ] **Vite Build:** FAILED\n"
		os.WriteFile(smokeFile, []byte(reportHeader+"\n\n# ABORTED: Build failed"), 0644)
		return fmt.Errorf("pre-smoke build failed")
	} else {
		reportHeader += "- [x] **Vite Build:** PASSED\n"
	}

	reportHeader += "\n---\n"
	os.WriteFile(smokeFile, []byte(reportHeader), 0644)

	// START SERVER
	browser.CleanupPort(port)
	// 'go' will now resolve via absolute path in PATH due to .env change
	cmd := exec.Command("go", "run", "cmd/main.go")
	cmd.Dir = pluginDir
	cmd.Env = os.Environ()
	
	logFile, _ := os.Create(filepath.Join(pluginDir, "smoke_server.log"))
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	
	if err := cmd.Start(); err != nil {
		fmt.Printf("   [ERROR] Failed to start wsl plugin: %v\n", err)
		return err
	}
	defer cmd.Process.Kill()

	if err := waitForPort(port, 30*time.Second); err != nil {
		fmt.Printf("   [ERROR] Host node timeout: %v\n", err)
		return err
	}

	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	fmt.Printf(">> [WSL] Plugin started. Access UI at: %s\n", url)

	wsURL, isNew, err := resolveChrome(0, true)
	if err != nil {
		fmt.Printf("   [ERROR] Chrome resolution failed: %v\n", err)
		return err
	}
	fmt.Printf(">> [WSL] Chrome WebSocket: %s\n", wsURL)

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
		case *cdruntime.EventConsoleAPICalled:
			msg := formatConsoleArgs(ev.Args)
			mu.Lock()
			currentLogs = append(currentLogs, fmt.Sprintf("[%s] %s", ev.Type, msg))
			mu.Unlock()
			fmt.Printf("   [BROWSER] [%s] %s\n", ev.Type, msg)
		}
	})

	stepCount := 0
	runStep := func(name string, actions chromedp.Action) error {
		fmt.Printf(">> [WSL] Step: %s\n", name)
		
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
		
		shotName := fmt.Sprintf("smoke_step_%d.png", stepCount)
		if len(buf) > 0 {
			os.WriteFile(filepath.Join(pluginDir, shotName), buf, 0644)
		}

		f, _ := os.OpenFile(smokeFile, os.O_APPEND|os.O_WRONLY, 0644)
		defer f.Close()
		fmt.Fprintf(f, "\n### %d. %s\n\n![%s](%s)\n\n---", stepCount+1, name, name, shotName)
		stepCount++
		return nil
	}

	// Initial Navigation
	if err := chromedp.Run(ctx, 
		chromedp.EmulateViewport(1280, 800),
		chromedp.Navigate(fmt.Sprintf("http://127.0.0.1:%d", port)),
		chromedp.WaitVisible(`#wsl-table`, chromedp.ByQuery),
	); err != nil { 
		fmt.Printf("   [ERROR] Initial navigation failed: %v\n", err)
		return err 
	}

	if err := runStep("WSL Table Rendering", chromedp.WaitVisible("#node-rows", chromedp.ByQuery)); err != nil { return err }
	
	testNode := "smoke-test-node"
	if err := runStep("Spawn WSL Node", chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(fmt.Sprintf(`window.prompt = () => "%s";`, testNode), nil).Do(ctx)
		}),
		chromedp.Click(`#start-node`, chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			start := time.Now()
			for time.Since(start) < 90*time.Second {
				var isRunning bool
				_ = chromedp.Evaluate(fmt.Sprintf(`document.body.innerText.includes("%s") && document.body.innerText.includes("RUNNING")`, testNode), &isRunning).Do(ctx)
				if isRunning { return nil }
				time.Sleep(2 * time.Second)
			}
			return fmt.Errorf("timeout waiting for %s to reach RUNNING state", testNode)
		}),
	}); err != nil { return err }

	if err := runStep("Verify Running & Stats", chromedp.ActionFunc(func(ctx context.Context) error {
		start := time.Now()
		for time.Since(start) < 30*time.Second {
			var statsReady bool
			_ = chromedp.Evaluate(fmt.Sprintf(`
				(function() {
					const rows = Array.from(document.querySelectorAll("#node-rows tr"));
					const row = rows.find(r => r.innerText.includes("%s"));
					if (!row) return false;
					const cells = Array.from(row.querySelectorAll("td"));
					if (cells.length < 5) return false;
					const mem = cells[3].innerText;
					const disk = cells[4].innerText;
					return mem !== "--" && disk !== "--";
				})()
			`, testNode), &statsReady).Do(ctx)
			if statsReady { return nil }
			time.Sleep(2 * time.Second)
		}
		return fmt.Errorf("timeout waiting for stats for %s", testNode)
	})); err != nil { return err }

	if err := runStep("Stop Node", chromedp.Tasks{
		chromedp.Click(fmt.Sprintf(`button[aria-label="Stop Node %s"]`, testNode), chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			start := time.Now()
			for time.Since(start) < 20*time.Second {
				var isStopped bool
				_ = chromedp.Evaluate(fmt.Sprintf(`document.body.innerText.includes("STOPPED") && document.body.innerText.includes("%s")`, testNode), &isStopped).Do(ctx)
				if isStopped { return nil }
				time.Sleep(1 * time.Second)
			}
			return fmt.Errorf("timeout waiting for %s to stop", testNode)
		}),
	}); err != nil { return err }

	if err := runStep("Delete Node", chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(`window.confirm = () => true;`, nil).Do(ctx)
		}),
		chromedp.Click(fmt.Sprintf(`button[aria-label="Delete Node %s"]`, testNode), chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			start := time.Now()
			for time.Since(start) < 20*time.Second {
				var found bool
				_ = chromedp.Evaluate(fmt.Sprintf(`document.body.innerText.includes("%s")`, testNode), &found).Do(ctx)
				if !found { return nil }
				time.Sleep(1 * time.Second)
			}
			return fmt.Errorf("timeout waiting for %s to be deleted", testNode)
		}),
	}); err != nil { return err }

	fmt.Printf(">> [WSL] Smoke: COMPLETE. Report at %s\n", smokeFile)
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

func formatConsoleArgs(args []*cdruntime.RemoteObject) string {
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