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

	"dialtone/dev/browser"
	chrome_app "dialtone/dev/plugins/chrome/app"
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
				lastStack = stack
			}
			mu.Unlock()
			fmt.Printf("   [BROWSER] [%s] %s\n", ev.Type, msg)
		}
	})

	testResults := []smokeEntry{}

	// Incremental report initialization
	os.WriteFile(smokeFile, []byte("# Nix Robust Smoke Test Report\n\n**Started:** "+time.Now().Format(time.RFC1123)+"\n"), 0644)

	appendToReport := func(res smokeEntry) {
		f, err := os.OpenFile(smokeFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer f.Close()

		sm := []string{}
		sm = append(sm, fmt.Sprintf("\n### %s", res.name))
		sm = append(sm, fmt.Sprintf("\n- **Status:** %s", res.status))
		sm = append(sm, fmt.Sprintf("- **Conditions:** %s", res.conditions))

		if res.errorMsg != "" {
			sm = append(sm, fmt.Sprintf("- **Error:** `%s`", res.errorMsg))
			if res.stackTrace != "" {
				sm = append(sm, "\n**Stack Trace:**\n```\n"+res.stackTrace+"\n```")
			}
		}

		sm = append(sm, "\n#### Visual proof")
		sm = append(sm, fmt.Sprintf("![%s](%s)", res.name, res.screenshot))

		sm = append(sm, "\n#### Last 10 Console Logs")
		sm = append(sm, "```")
		start := 0
		if len(res.logs) > 10 {
			start = len(res.logs) - 10
		}
		for i := start; i < len(res.logs); i++ {
			sm = append(sm, res.logs[i])
		}
		sm = append(sm, "```")
		sm = append(sm, "\n---")
		f.WriteString(strings.Join(sm, "\n"))
	}

	// POLLING HELPER
	pollJS := func(condition string, timeout time.Duration) chromedp.Action {
		return chromedp.ActionFunc(func(ctx context.Context) error {
			start := time.Now()
			for time.Since(start) < timeout {
				var ok bool
				err := chromedp.Run(ctx, chromedp.Evaluate(condition, &ok))
				if err == nil && ok {
					return nil
				}
				time.Sleep(100 * time.Millisecond)
			}
			return fmt.Errorf("timeout polling JS condition: %s", condition)
		})
	}

	// MANUAL TIMEOUT WRAPPER
	runStep := func(name string, conditions string, actions chromedp.Action) error {
		fmt.Printf(">> [NIX] Step: %s (10s limit)\n", name)

		mu.Lock()
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
		_ = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			b, err := page.CaptureScreenshot().Do(ctx)
			if err != nil {
				return err
			}
			buf = b
			return nil
		}))

		shotName := fmt.Sprintf("smoke_step_%d.png", len(testResults))
		if len(buf) > 0 {
			os.WriteFile(filepath.Join(pluginDir, shotName), buf, 0644)
		}

		status := "✅ PASSED"
		errDetail := ""
		if err != nil {
			status = "❌ FAILED"
			errDetail = err.Error()
		}

		mu.Lock()
		logsCopy := make([]string, len(currentLogs))
		copy(logsCopy, currentLogs)

		entry := smokeEntry{
			name:       name,
			conditions: conditions,
			status:     status,
			errorMsg:   errDetail,
			stackTrace: lastStack,
			screenshot: shotName,
			logs:       logsCopy,
		}
		testResults = append(testResults, entry)
		mu.Unlock()

		appendToReport(entry)

		if err != nil {
			return fmt.Errorf("smoke test stopped at step '%s': %v", name, err)
		}
		return nil
	}

	// NAVIGATION WRAPPER
	navigate := func(id string) chromedp.Action {
		return chromedp.Tasks{
			// Wait for the navigation function to be available using a Go-side poll
			chromedp.ActionFunc(func(ctx context.Context) error {
				for i := 0; i < 50; i++ {
					var exists bool
					err := chromedp.Run(ctx, chromedp.Evaluate(`typeof window.navigateTo === 'function'`, &exists))
					if err == nil && exists {
						return nil
					}
					time.Sleep(100 * time.Millisecond)
				}
				return fmt.Errorf("window.navigateTo not found after 5s")
			}),
			chromedp.Evaluate(fmt.Sprintf("window.navigateTo('%s', false)", id), nil),
			pollJS(fmt.Sprintf(`document.querySelector("#%s").classList.contains("is-visible")`, id), 2*time.Second),
			chromedp.Evaluate(`new Promise(resolve => {
				const handler = (e) => {
					if (e.detail.sectionId === '`+id+`') {
						window.removeEventListener('section-nav-complete', handler);
						resolve();
					}
				};
				window.addEventListener('section-nav-complete', handler);
				// Timeout safety
				setTimeout(resolve, 1000);
			})`, nil),
		}
	}

	// TEST SEQUENCE

	if err := runStep("1. Verify Browser Error Capture", "Navigate to home and verify captured log", chromedp.Tasks{
		chromedp.EmulateViewport(1280, 800),
		chromedp.Navigate(fmt.Sprintf("http://127.0.0.1:%d", port)),
		pollJS(`document.querySelector("#nix-hero").classList.contains("is-visible")`, 5*time.Second),
		chromedp.WaitVisible("#viz-container", chromedp.ByQuery),
		chromedp.Evaluate(`console.error('[SMOKE-VERIFY-ERR] Log pipeline verified')`, nil),
	}); err != nil {
		return err
	}

	if err := runStep("2. Hero Section Validation", "Viz container and marketing overlay visible", chromedp.Tasks{
		chromedp.WaitVisible("#viz-container", chromedp.ByQuery),
		chromedp.WaitVisible("#nix-hero.is-visible .marketing-overlay", chromedp.ByQuery),
	}); err != nil {
		return err
	}

	if err := runStep("3. Documentation Section Validation", "Navigate to nix-docs and verify content", chromedp.Tasks{
		navigate("nix-docs"),
		chromedp.WaitVisible("#nix-docs.is-visible", chromedp.ByQuery),
		chromedp.WaitVisible("#nix-docs h1", chromedp.ByQuery),
		chromedp.WaitVisible("#nix-docs ul", chromedp.ByQuery),
	}); err != nil {
		return err
	}

	if err := runStep("4. Navigate to Nix Table and Verify Rendering", "Switch to nix-table and verify fullscreen layout + hidden header/menu", chromedp.Tasks{
		navigate("nix-table"),
		chromedp.WaitVisible("#nix-table.is-visible", chromedp.ByQuery),
		pollJS(`getComputedStyle(document.querySelector('.header-title')).opacity === '0' || getComputedStyle(document.querySelector('.header-title')).visibility === 'hidden'`, 2*time.Second),
		pollJS(`getComputedStyle(document.getElementById('global-menu')).display === 'none'`, 2*time.Second),
		chromedp.WaitVisible(".explorer-container", chromedp.ByQuery),
		chromedp.WaitVisible("#start-node", chromedp.ByQuery),
	}); err != nil {
		return err
	}

	if err := runStep("5. Spawn Two Nix Nodes", "Two nodes appear in table", chromedp.Tasks{
		chromedp.Click(`#start-node`, chromedp.ByQuery),
		chromedp.WaitVisible("#proc-1", chromedp.ByQuery),
		chromedp.Click(`#start-node`, chromedp.ByQuery),
		chromedp.WaitVisible("#proc-2", chromedp.ByQuery),
		chromedp.WaitVisible(".node-row", chromedp.ByQuery),
	}); err != nil {
		return err
	}

	if err := runStep("6. Selective Termination (proc-1)", "proc-1 status changes to STOPPED", chromedp.Tasks{
		chromedp.WaitVisible("#proc-1 .stop-btn", chromedp.ByQuery),
		chromedp.Click("#proc-1 .stop-btn", chromedp.ByQuery),
		chromedp.WaitVisible("#proc-1 .status-badge[data-status-text='stopped']", chromedp.ByQuery),
	}); err != nil {
		return err
	}

	if err := runStep("7. Verify proc-2 Persistence", "proc-2 remains RUNNING", chromedp.Tasks{
		chromedp.WaitVisible("#proc-2 .status-badge[data-status-text='running']", chromedp.ByQuery),
	}); err != nil {
		return err
	}

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
