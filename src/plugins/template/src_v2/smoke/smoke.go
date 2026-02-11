package smoke

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

type consoleEntry struct {
	level   string
	message string
}

type preflightResult struct {
	name   string
	status string
	log    string
}

type stepResult struct {
	name       string
	status     string
	screenshot string
	logs       []consoleEntry
	err        error
}

func Run(versionDir string) error {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "template", versionDir)
	smokeDir := filepath.Join(pluginDir, "smoke")
	os.MkdirAll(smokeDir, 0755)

	smokeFile := filepath.Join(smokeDir, "SMOKE.md")
	smokeLogFile := filepath.Join(smokeDir, "smoke.log")
	port := 8080

	// Initialize log file
	logF, err := os.Create(smokeLogFile)
	if err != nil {
		return fmt.Errorf("failed to create smoke.log: %v", err)
	}
	defer logF.Close()

	// Multi-writer for stdout and file
	mw := io.MultiWriter(os.Stdout, logF)
	logMsg := func(format string, a ...interface{}) {
		fmt.Fprintf(mw, format, a...)
	}

	logMsg("[SMOKE] START for %s\n", versionDir)

	var preflightResults []preflightResult
	
	// Phase 1: Preflight (Install, Lint, Build)
	preflightErr := runPreflight(cwd, versionDir, &preflightResults, mw)
	
	if preflightErr != nil {
		logMsg("[SMOKE] Preflight encountered issues, continuing to capture what we can.\n")
	}

	// Phase 2: Start Server
	browser.CleanupPort(port)
	cmd := exec.Command("go", "run", "cmd/main.go")
	cmd.Dir = pluginDir
	serverLogFile, _ := os.Create(filepath.Join(smokeDir, "smoke_server.log"))
	cmd.Stdout = serverLogFile
	cmd.Stderr = serverLogFile

	logMsg("[SMOKE] Starting plugin server on port %d...\n", port)
	if err := cmd.Start(); err != nil {
		writeFinalReport(smokeFile, preflightResults, nil, nil, nil)
		return fmt.Errorf("failed to start template plugin: %v", err)
	}
	defer cmd.Process.Kill()

	if err := waitForPort(port, 15*time.Second); err != nil {
		writeFinalReport(smokeFile, preflightResults, nil, nil, nil)
		return fmt.Errorf("server timeout: %v", err)
	}
	logMsg("[SMOKE] Server ready.\n")

	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	wsURL, isNew, err := resolveChrome(0, true)
	if err != nil {
		writeFinalReport(smokeFile, preflightResults, nil, nil, nil)
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
	var entries []consoleEntry
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			msg := formatConsoleArgs(ev.Args)
			mu.Lock()
			entries = append(entries, consoleEntry{level: string(ev.Type), message: msg})
			mu.Unlock()
			logMsg("   [BROWSER] [%s] %s\n", ev.Type, msg)
		}
	})

	logMsg("[SMOKE] Navigating to %s...\n", url)
	// Navigation & Trigger Proof of Life
	if err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1280, 800),
		chromedp.Navigate(url),
		chromedp.Evaluate(`console.error('[PROOFOFLIFE] Intentional Browser Test Error')`, nil),
	); err != nil {
		writeFinalReport(smokeFile, preflightResults, nil, nil, nil)
		return err
	}

	mu.Lock()
	entries = append(entries, consoleEntry{level: "error", message: "[PROOFOFLIFE] Intentional Go Test Error"})
	mu.Unlock()

	// Wait for error to be captured
	time.Sleep(500 * time.Millisecond)

	var stepResults []stepResult
	var lastLogIdx int
	mu.Lock()
	lastLogIdx = 0 // Capture everything from the beginning in the first step
	mu.Unlock()

	runStep := func(name string, actions chromedp.Action) {
		logMsg("[SMOKE] Step Start: %s\n", name)
		err := chromedp.Run(ctx, actions)
		
		var buf []byte
		_ = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			b, _ := page.CaptureScreenshot().Do(ctx)
			buf = b
			return nil
		}))

		shotName := fmt.Sprintf("smoke_step_%d.png", len(stepResults)+1)
		if len(buf) > 0 {
			os.WriteFile(filepath.Join(smokeDir, shotName), buf, 0644)
		}

		mu.Lock()
		stepLogs := entries[lastLogIdx:]
		lastLogIdx = len(entries)
		mu.Unlock()

		status := "PASS"
		if err != nil {
			status = "FAIL"
			logMsg("[SMOKE] Step FAILED: %s | Error: %v\n", name, err)
		} else {
			logMsg("[SMOKE] Step PASSED: %s\n", name)
		}
		stepResults = append(stepResults, stepResult{
			name:       name,
			status:     status,
			screenshot: shotName,
			logs:       stepLogs,
			err:        err,
		})
	}

	runStep("Hero Section Validation", dialtest.WaitForAriaLabel("Home Section"))
	runStep("Documentation Section Validation", dialtest.NavigateToSection("docs", "Docs Section"))
	runStep("Table Section Validation", dialtest.NavigateToSection("table", "Table Section"))
	runStep("Verify Header Hidden on Table", dialtest.AssertElementHidden(".header-title"))
	runStep("Settings Section Validation", dialtest.NavigateToSection("settings", "Settings Section"))
	runStep("Return Home", dialtest.NavigateToSection("home", "Home Section"))

	// Collect POL and Real Errors
	var polEntries []consoleEntry
	var realErrors []consoleEntry
	mu.Lock()
	for _, e := range entries {
		if strings.Contains(e.message, "[PROOFOFLIFE]") {
			polEntries = append(polEntries, e)
		} else if e.level == "error" || e.level == "exception" {
			realErrors = append(realErrors, e)
		}
	}
	mu.Unlock()

	logMsg("[SMOKE] Generating markdown report...\n")
	writeFinalReport(smokeFile, preflightResults, polEntries, realErrors, stepResults)

	logMsg("[SMOKE] COMPLETE. Report at %s\n", smokeFile)
	return nil
}

func runPreflight(repoRoot, versionDir string, results *[]preflightResult, mw io.Writer) error {
	uiDir := filepath.Join(repoRoot, "src", "plugins", "template", versionDir, "ui")
	steps := []struct {
		name string
		cmd  string
		args []string
	}{
		{"Install", "bun", []string{"install"}},
		{"Lint", "bun", []string{"run", "lint"}},
		{"Build", "bun", []string{"run", "build"}},
	}

	var firstErr error
	for _, s := range steps {
		fmt.Fprintf(mw, "[SMOKE] Preflight: %s...\n", s.name)
		out, err := runCommandCapture(uiDir, s.cmd, s.args...)
		status := "✅ PASSED"
		if err != nil {
			status = "❌ FAILED"
			if firstErr == nil {
				firstErr = err
			}
		}
		*results = append(*results, preflightResult{s.name, status, string(out)})
	}
	return firstErr
}

func writeFinalReport(smokeFile string, preflight []preflightResult, pol []consoleEntry, real []consoleEntry, steps []stepResult) {
	var buf bytes.Buffer
	buf.WriteString("# Template Plugin Smoke Test Report\n\n")
	buf.WriteString(fmt.Sprintf("**Generated at:** %s\n\n", time.Now().Format(time.RFC1123)))

	// 1. Expected Errors (Proof of Life)
	buf.WriteString("## 1. Expected Errors (Proof of Life)\n\n")
	if len(pol) == 0 {
		buf.WriteString("❌ ERROR: Proof of Life logs missing! Logging pipeline may be broken.\n")
	} else {
		buf.WriteString("| Level | Message | Status |\n")
		buf.WriteString("|---|---|---|\n")
		for _, e := range pol {
			buf.WriteString(fmt.Sprintf("| %s | %s | ✅ CAPTURED |\n", e.level, e.message))
		}
	}
	buf.WriteString("\n---\n\n")

	// 2. Real Errors & Warnings
	buf.WriteString("## 2. Real Errors & Warnings\n\n")
	if len(real) == 0 {
		buf.WriteString("✅ No actual issues detected.\n")
	} else {
		for _, e := range real {
			buf.WriteString(fmt.Sprintf("### [%s]\n```text\n%s\n```\n", e.level, e.message))
		}
	}
	buf.WriteString("\n---\n\n")

	// 3. Preflight: Environment & Build
	buf.WriteString("## 3. Preflight: Environment & Build\n")
	for _, p := range preflight {
		buf.WriteString(fmt.Sprintf("\n### %s: %s\n\n```text\n%s\n```\n", p.name, p.status, strings.TrimSpace(p.log)))
	}
	buf.WriteString("\n---\n\n")

	// 4. UI & Interactivity
	buf.WriteString("## 4. UI & Interactivity\n")
	
	// Lifecycle Verification
	buf.WriteString("\n### Lifecycle Verification Summary\n\n")
	verifyLifecycle(&buf, steps)

	for i, s := range steps {
		icon := "✅"
		if s.status == "FAIL" {
			icon = "❌"
		}
		buf.WriteString(fmt.Sprintf("\n### %d. %s: %s %s\n\n", i+1, s.name, s.status, icon))
		if s.err != nil {
			buf.WriteString(fmt.Sprintf("**Error:** `%v`\n\n", s.err))
		}
		if len(s.logs) > 0 {
			buf.WriteString("**Console Logs:**\n```text\n")
			for _, l := range s.logs {
				buf.WriteString(fmt.Sprintf("[%s] %s\n", l.level, l.message))
			}
			buf.WriteString("```\n\n")
		}
		if s.screenshot != "" {
			buf.WriteString(fmt.Sprintf("![%s](%s)\n\n", s.name, s.screenshot))
		}
		buf.WriteString("---\n")
	}

	os.WriteFile(smokeFile, buf.Bytes(), 0644)
}

func verifyLifecycle(buf *bytes.Buffer, steps []stepResult) {
	events := []string{"LOADING", "LOADED", "START", "RESUME", "PAUSE", "AWAKE", "SLEEP"}
	found := make(map[string]bool)
	
	for _, step := range steps {
		for _, log := range step.logs {
			for _, event := range events {
				if strings.Contains(log.message, event) {
					found[event] = true
				}
			}
		}
	}

	buf.WriteString("| Event | Status | Description |\n")
	buf.WriteString("|---|---|---|\n")
	
	eventMeta := []struct{ name, desc string }{
		{"LOADING", "Section chunk fetching initiated"},
		{"LOADED", "Section code loaded into memory"},
		{"START", "Section component initialized"},
		{"RESUME / AWAKE", "Animation loop active and visible"},
		{"PAUSE / SLEEP", "Animation loop suspended when off-screen"},
	}

	for _, meta := range eventMeta {
		status := "❌ MISSING"
		// Check for variants
		if strings.Contains(meta.name, " / ") {
			parts := strings.Split(meta.name, " / ")
			if found[parts[0]] || found[parts[1]] {
				status = "✅ CAPTURED"
			}
		} else if found[meta.name] {
			status = "✅ CAPTURED"
		}
		buf.WriteString(fmt.Sprintf("| %s | %s | %s |\n", meta.name, status, meta.desc))
	}
	buf.WriteString("\n")
}

func runCommandCapture(dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.Bytes(), err
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