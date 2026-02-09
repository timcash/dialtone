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

type smokeEntry struct {
	name       string
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
		return fmt.Errorf("failed to start nix plugin: %v", err)
	}
	defer cmd.Process.Kill()

	if err := waitForPort(port, 15*time.Second); err != nil {
		return fmt.Errorf("host node timeout: %v", err)
	}

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

	var mu sync.Mutex
	var currentLogs []string
	var lastErrorMsg string
	var lastStack string
	
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
			currentLogs = append(currentLogs, fmt.Sprintf("[%s] %s", ev.Type, msg))
			if ev.Type == "error" {
				lastErrorMsg = msg
				lastStack = stack
			}
			mu.Unlock()
			fmt.Printf("   [BROWSER] [%s] %s\n", ev.Type, msg)
		}
	})

	testResults := []smokeEntry{}

	// MANUAL TIMEOUT WRAPPER (Strict 10s per test as requested)
	runStep := func(name string, actions chromedp.Action) error {
		fmt.Printf(">> [NIX] Step: %s (10s limit)\n", name)
		
		mu.Lock()
		lastErrorMsg = ""
		lastStack = ""
		mu.Unlock()

		errChan := make(chan error, 1)
		go func() {
			errChan <- chromedp.Run(ctx, actions)
		}()

		var err error
		select {
		case err = <-errChan:
			if err != nil {
				fmt.Printf("   [ERROR] Action failed: %v\n", err)
			}
		case <-time.After(10 * time.Second):
			err = fmt.Errorf("manual step timeout (10s)")
			fmt.Printf("   [ERROR] Step TIMED OUT\n")
		}

		var buf []byte
		_ = chromedp.Run(ctx, chromedp.Screenshot("#app", &buf, chromedp.NodeVisible))
		
		shotName := fmt.Sprintf("smoke_step_%d.png", len(testResults))
		if len(buf) > 0 {
			os.WriteFile(filepath.Join(pluginDir, shotName), buf, 0644)
		}

		status := "✅ PASSED"
		errDetail := ""
		if err != nil {
			status = "❌ FAILED"
			errDetail = err.Error()
		} else if lastErrorMsg != "" && strings.Contains(name, "Verify Browser Error") {
			errDetail = lastErrorMsg
		}

		mu.Lock()
		logsCopy := make([]string, len(currentLogs))
		copy(logsCopy, currentLogs)
		
		testResults = append(testResults, smokeEntry{
			name:       name,
			status:     status,
			errorMsg:   errDetail,
			stackTrace: lastStack,
			screenshot: shotName,
			logs:       logsCopy,
		})
		mu.Unlock()
		return err
	}

	// TEST SEQUENCE
	
	runStep("1. Verify Browser Error Capture", chromedp.Tasks{
		chromedp.Navigate(fmt.Sprintf("http://127.0.0.1:%d", port)),
		chromedp.WaitVisible("#viz-container", chromedp.ByQuery),
		chromedp.Evaluate(`console.error('[SMOKE-VERIFY-ERR] Log pipeline verified')`, nil),
		chromedp.Sleep(500 * time.Millisecond),
	})

	runStep("2. Hero Section Validation", chromedp.Tasks{
		chromedp.WaitVisible("#viz-container", chromedp.ByQuery),
		chromedp.WaitVisible(".marketing-overlay", chromedp.ByQuery),
	})

	runStep("3. Spawn Two Nix Nodes", chromedp.Tasks{
		chromedp.Evaluate(fmt.Sprintf("window.location.hash = 's-nixtable'"), nil),
		chromedp.WaitVisible("#start-node", chromedp.ByQuery),
		chromedp.Click(`button[aria-label="Spawn Nix Node"]`, chromedp.ByQuery),
		chromedp.WaitVisible("#proc-1", chromedp.ByQuery),
		chromedp.Click(`button[aria-label="Spawn Nix Node"]`, chromedp.ByQuery),
		chromedp.WaitVisible("#proc-2", chromedp.ByQuery),
	})

	runStep("4. Selective Termination (proc-1)", chromedp.Tasks{
		chromedp.Click(`button[aria-label="Stop Node proc-1"]`, chromedp.ByQuery),
		chromedp.WaitVisible("#proc-1 .status-badge[data-status-text='stopped']", chromedp.ByQuery),
	})

	runStep("5. Verify proc-2 Persistence", chromedp.Tasks{
		chromedp.WaitVisible("#proc-2 .status-badge[data-status-text='running']", chromedp.ByQuery),
	})

	// GENERATE SMOKE.md
	fmt.Println(">> [NIX] Smoke: generating detailed report...")
	var sm []string
	sm = append(sm, "# Nix Robust Smoke Test Report")
	sm = append(sm, fmt.Sprintf("\n**Generated:** %s", time.Now().Format(time.RFC1123)))
	
	for i, res := range testResults {
		sm = append(sm, fmt.Sprintf("\n### Test %d: %s", i+1, res.name))
		sm = append(sm, fmt.Sprintf("\n- **Status:** %s", res.status))
		
		if res.errorMsg != "" {
			sm = append(sm, fmt.Sprintf("- **Error:** `%s`", res.errorMsg))
			if res.stackTrace != "" {
				sm = append(sm, "\n**Stack Trace:**\n```\n"+res.stackTrace+"\n```")
			}
		}

		sm = append(sm, "\n#### Visual proof")
		sm = append(sm, fmt.Sprintf("![Step %d](%s)", i+1, res.screenshot))

		sm = append(sm, "\n#### Last 20 Console Logs")
		sm = append(sm, "```")
		for _, l := range res.logs {
			sm = append(sm, l)
		}
		sm = append(sm, "```")
		sm = append(sm, "\n---")
	}

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
		if arg == nil { continue }
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
