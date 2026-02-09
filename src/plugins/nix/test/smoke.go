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
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type consoleEntry struct {
	level   string
	message string
	stack   string
}

func RunSmoke(versionDir string, timeoutSec int) error {
	fmt.Printf(">> [NIX] Smoke: START for %s (timeout: %ds)\n", versionDir, timeoutSec)

	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "nix", versionDir)
	smokeFile := filepath.Join(pluginDir, "SMOKE.md")
	port := 8080

	// 0. Ensure dist exists
	uiDist := filepath.Join(pluginDir, "ui", "dist")
	if _, err := os.Stat(uiDist); os.IsNotExist(err) {
		return fmt.Errorf("UI dist folder not found at %s. Please run build first", uiDist)
	}

	// 1. Cleanup and Start Host Node
	fmt.Printf(">> [NIX] Smoke: cleanup port %d and starting host node...\n", port)
	browser.CleanupPort(port)

	cmd := exec.Command("go", "run", "cmd/main.go")
	cmd.Dir = pluginDir
	logFile, _ := os.Create(filepath.Join(pluginDir, "smoke_server.log"))
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start nix plugin: %v", err)
	}
	defer func() {
		fmt.Println(">> [NIX] Smoke: killing host node")
		cmd.Process.Kill()
	}()

	// 2. Wait for port 8080 with feedback
	fmt.Printf(">> [NIX] Smoke: waiting for port %d to open...\n", port)
	if err := waitForPort(port, 15*time.Second); err != nil {
		return fmt.Errorf("host node failed to listen on %d: %v", port, err)
	}
	fmt.Printf(">> [NIX] Smoke: host node is alive on %d\n", port)

	// 3. Connect to Chrome
	fmt.Println(">> [NIX] Smoke: resolving chrome instance...")
	wsURL, isNew, err := resolveChrome(0, true) // auto-port, headless
	if err != nil {
		return fmt.Errorf("failed to resolve chrome: %v", err)
	}
	fmt.Printf(">> [NIX] Smoke: connected to chrome at %s (new: %v)\n", wsURL, isNew)

	defer func() {
		if isNew {
			fmt.Println(">> [NIX] Smoke: cleaning up launched chrome")
			exec.Command("./dialtone.sh", "chrome", "kill", "all").Run()
		}
	}()

	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// Console logs collection
	var mu sync.Mutex
	var consoleLogs []consoleEntry
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			msg := formatConsoleArgs(ev.Args)
			stack := ""
			if ev.StackTrace != nil {
				for _, f := range ev.StackTrace.CallFrames {
					stack += fmt.Sprintf("  %s (%s:%d:%d)\n", f.FunctionName, f.URL, f.LineNumber, f.ColumnNumber)
				}
			}
			mu.Lock()
			consoleLogs = append(consoleLogs, consoleEntry{
				level:   string(ev.Type),
				message: msg,
				stack:   stack,
			})
			mu.Unlock()
			fmt.Printf("   [BROWSER] [%s] %s\n", ev.Type, msg)
		case *runtime.EventExceptionThrown:
			msg := ev.ExceptionDetails.Text
			if ev.ExceptionDetails.Exception != nil {
				msg = ev.ExceptionDetails.Exception.Description
			}
			mu.Lock()
			consoleLogs = append(consoleLogs, consoleEntry{
				level:   "exception",
				message: msg,
			})
			mu.Unlock()
			fmt.Printf("   [BROWSER] [EXCEPTION] %s\n", msg)
		}
	})

	// 4. Test UI Flow
	fmt.Println(">> [NIX] Smoke: Step 1 - Navigating to UI...")
	var buf1, buf2 []byte
	var procID string

	err = chromedp.Run(ctx,
		chromedp.Navigate(fmt.Sprintf("http://127.0.0.1:%d", port)),
		chromedp.WaitVisible("#status", chromedp.ByQuery),
		chromedp.Evaluate(`console.log('[SMOKE] Navigation successful')`, nil),
	)
	if err != nil {
		return fmt.Errorf("failed to navigate or find #status: %v", err)
	}

	fmt.Println(">> [NIX] Smoke: Step 2 - Triggering Proof-of-Life Error...")
	chromedp.Run(ctx, chromedp.Evaluate(`console.error('[ERROR-PING] Intentional smoke test error')`, nil))

	fmt.Println(">> [NIX] Smoke: Step 3 - Starting Nix Sub-Process...")
	err = chromedp.Run(ctx,
		chromedp.Click("#start-proc", chromedp.ByQuery),
		chromedp.WaitVisible(".proc-container", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Give it a moment to show logs
		chromedp.Screenshot("#app", &buf1, chromedp.NodeVisible),
		chromedp.AttributeValue(".proc-header button", "id", &procID, nil, chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Errorf("failed to start sub-process or verify UI: %v", err)
	}
	os.WriteFile(filepath.Join(pluginDir, "smoke_step1_active.png"), buf1, 0644)

	cleanID := strings.TrimPrefix(procID, "stop-")
	fmt.Printf(">> [NIX] Smoke: Step 4 - Stopping Sub-Process (%s)...\n", cleanID)

	err = chromedp.Run(ctx,
		chromedp.Click("#"+procID, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Screenshot("#app", &buf2, chromedp.NodeVisible),
	)
	if err != nil {
		return fmt.Errorf("failed to stop sub-process: %v", err)
	}
	os.WriteFile(filepath.Join(pluginDir, "smoke_step2_stopped.png"), buf2, 0644)

	// 5. Generate SMOKE.md
	fmt.Println(">> [NIX] Smoke: generating SMOKE.md report...")

	var sm []string
	sm = append(sm, "# Nix Smoke Test Report")
	sm = append(sm, fmt.Sprintf("\n**Generated:** %s", time.Now().Format(time.RFC1123)))
	sm = append(sm, "\n## Status: ✅ PASSED")
	sm = append(sm, fmt.Sprintf("\n- **Host Node**: `http://127.0.0.1:%d`", port))
	sm = append(sm, fmt.Sprintf("- **Version**: `%s`", versionDir))
	sm = append(sm, fmt.Sprintf("- **Internal Timeout**: `%ds`", timeoutSec))

	sm = append(sm, "\n## Visual Evidence")
	sm = append(sm, "\n| Started & Logging | Stopped |")
	sm = append(sm, "|---|---|")
	sm = append(sm, "| ![Step 1](smoke_step1_active.png) | ![Step 2](smoke_step2_stopped.png) |")

	sm = append(sm, "\n## Browser Console Logs")
	sm = append(sm, "\n```")
	mu.Lock()
	for _, l := range consoleLogs {
		sm = append(sm, fmt.Sprintf("[%s] %s", l.level, l.message))
		if l.stack != "" {
			sm = append(sm, l.stack)
		}
	}
	mu.Unlock()
	sm = append(sm, "```")

	sm = append(sm, "\n## Host Server Logs")
	sm = append(sm, "\n```")
	hLogs, _ := os.ReadFile(filepath.Join(pluginDir, "smoke_server.log"))
	sm = append(sm, string(hLogs))
	sm = append(sm, "```")

	os.WriteFile(smokeFile, []byte(strings.Join(sm, "\n")), 0644)

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
	// Try existing
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

	// Launch new
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
