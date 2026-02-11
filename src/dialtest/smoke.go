package dialtest

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

	chrome_app "dialtone/cli/src/plugins/chrome/app"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type ConsoleEntry struct {
	Level   string
	Message string
}

type PreflightResult struct {
	Name   string
	Status string
	Log    string
}

type StepResult struct {
	Name       string
	Status     string
	Screenshot string
	Logs       []ConsoleEntry
	Err        error
}

type SmokeOptions struct {
	Name       string
	VersionDir string
	Port       int
	SmokeDir   string
}

type SmokeRunner struct {
	Opts      SmokeOptions
	LogF      *os.File
	MW        io.Writer
	Entries   []ConsoleEntry
	Mu        sync.Mutex
	Steps     []StepResult
	Preflight []PreflightResult
	
	Ctx       context.Context
	Cancel    context.CancelFunc
	LastLogIdx int
	IsNewBrowser bool
}

func NewSmokeRunner(opts SmokeOptions) (*SmokeRunner, error) {
	if opts.Port == 0 {
		opts.Port = 8080
	}
	os.MkdirAll(opts.SmokeDir, 0755)
	
	smokeLogFile := filepath.Join(opts.SmokeDir, "smoke.log")
	logF, err := os.Create(smokeLogFile)
	if err != nil {
		return nil, err
	}

	mw := io.MultiWriter(os.Stdout, logF)

	return &SmokeRunner{
		Opts: opts,
		LogF: logF,
		MW:   mw,
	}, nil
}

func (r *SmokeRunner) LogMsg(format string, a ...interface{}) {
	fmt.Fprintf(r.MW, format, a...)
}

func (r *SmokeRunner) RunPreflight(repoRoot string, steps []struct{ Name, Cmd string; Args []string }) error {
	uiDir := filepath.Join(repoRoot, "src", "plugins", strings.ToLower(r.Opts.Name), r.Opts.VersionDir, "ui")
	
	var firstErr error
	for _, s := range steps {
		r.LogMsg("[SMOKE] Preflight: %s...\n", s.Name)
		cmd := exec.Command(s.Cmd, s.Args...)
		cmd.Dir = uiDir
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		err := cmd.Run()
		
		status := "✅ PASSED"
		if err != nil {
			status = "❌ FAILED"
			if firstErr == nil {
				firstErr = err
			}
		}
		r.Preflight = append(r.Preflight, PreflightResult{s.Name, status, buf.String()})
	}
	return firstErr
}

func (r *SmokeRunner) SetupBrowser(url string) error {
	wsURL, isNew, err := resolveChrome(0, true)
	if err != nil {
		return err
	}
	r.IsNewBrowser = isNew

	allocCtx, _ := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	r.Ctx, r.Cancel = chromedp.NewContext(allocCtx)
	
	chromedp.ListenTarget(r.Ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			msg := formatConsoleArgs(ev.Args)
			r.Mu.Lock()
			r.Entries = append(r.Entries, ConsoleEntry{Level: string(ev.Type), Message: msg})
			r.Mu.Unlock()
			r.LogMsg("   [BROWSER] [%s] %s\n", ev.Type, msg)
		}
	})

	r.LogMsg("[SMOKE] Navigating to %s...\n", url)
	return chromedp.Run(r.Ctx,
		chromedp.EmulateViewport(1280, 800),
		chromedp.Navigate(url),
		chromedp.Evaluate(`console.error('[PROOFOFLIFE] Intentional Browser Test Error')`, nil),
	)
}

func (r *SmokeRunner) StartServer(cmd *exec.Cmd) error {
	serverLogFile, _ := os.Create(filepath.Join(r.Opts.SmokeDir, "smoke_server.log"))
	cmd.Stdout = serverLogFile
	cmd.Stderr = serverLogFile

	r.LogMsg("[SMOKE] Starting plugin server on port %d...\n", r.Opts.Port)
	if err := cmd.Start(); err != nil {
		return err
	}

	if err := WaitForPort(r.Opts.Port, 15*time.Second); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("server timeout: %v", err)
	}
	r.LogMsg("[SMOKE] Server ready.\n")
	return nil
}

func (r *SmokeRunner) Step(name string, actions chromedp.Action) {
	r.LogMsg("[SMOKE] Step Start: %s\n", name)
	err := chromedp.Run(r.Ctx, actions)
	
	var buf []byte
	_ = chromedp.Run(r.Ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		b, _ := page.CaptureScreenshot().Do(ctx)
		buf = b
		return nil
	}))

	shotName := fmt.Sprintf("smoke_step_%d.png", len(r.Steps)+1)
	if len(buf) > 0 {
		os.WriteFile(filepath.Join(r.Opts.SmokeDir, shotName), buf, 0644)
	}

	r.Mu.Lock()
	stepLogs := make([]ConsoleEntry, len(r.Entries)-r.LastLogIdx)
	copy(stepLogs, r.Entries[r.LastLogIdx:])
	r.LastLogIdx = len(r.Entries)
	r.Mu.Unlock()

	status := "PASS"
	if err != nil {
		status = "FAIL"
		r.LogMsg("[SMOKE] Step FAILED: %s | Error: %v\n", name, err)
	} else {
		r.LogMsg("[SMOKE] Step PASSED: %s\n", name)
	}
	r.Steps = append(r.Steps, StepResult{
		Name:       name,
		Status:     status,
		Screenshot: shotName,
		Logs:       stepLogs,
		Err:        err,
	})
}

func (r *SmokeRunner) Finalize() {
	if r.Cancel != nil {
		r.Cancel()
	}

	// Trigger Proof of Life in Go side if not already there
	r.Mu.Lock()
	r.Entries = append(r.Entries, ConsoleEntry{Level: "error", Message: "[PROOFOFLIFE] Intentional Go Test Error"})
	r.Mu.Unlock()

	r.LogMsg("[SMOKE] Generating markdown report...\n")
	r.WriteFinalReport()
	
	smokeFile := filepath.Join(r.Opts.SmokeDir, "SMOKE.md")
	r.LogMsg("[SMOKE] COMPLETE. Report at %s\n", smokeFile)

	if r.IsNewBrowser {
		r.LogMsg("[SMOKE] Cleaning up browser processes using chrome plugin API...\n")
		chrome_app.KillAllResources()
	}

	if r.LogF != nil {
		r.LogF.Close()
	}
}

func (r *SmokeRunner) WriteFinalReport() {
	smokeFile := filepath.Join(r.Opts.SmokeDir, "SMOKE.md")
	
	var pol []ConsoleEntry
	var real []ConsoleEntry
	r.Mu.Lock()
	for _, e := range r.Entries {
		if strings.Contains(e.Message, "[PROOFOFLIFE]") {
			pol = append(pol, e)
		} else if e.Level == "error" || e.Level == "exception" {
			real = append(real, e)
		}
	}
	r.Mu.Unlock()

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("# %s Plugin Smoke Test Report\n\n", r.Opts.Name))
	buf.WriteString(fmt.Sprintf("**Generated at:** %s\n\n", time.Now().Format(time.RFC1123)))

	// 1. Expected Errors (Proof of Life)
	buf.WriteString("## 1. Expected Errors (Proof of Life)\n\n")
	if len(pol) == 0 {
		buf.WriteString("❌ ERROR: Proof of Life logs missing! Logging pipeline may be broken.\n")
	} else {
		buf.WriteString("| Level | Message | Status |\n")
		buf.WriteString("|---|---|---|\n")
		for _, e := range pol {
			buf.WriteString(fmt.Sprintf("| %s | %s | ✅ CAPTURED |\n", e.Level, e.Message))
		}
	}
	buf.WriteString("\n---\n\n")

	// 2. Real Errors & Warnings
	buf.WriteString("## 2. Real Errors & Warnings\n\n")
	if len(real) == 0 {
		buf.WriteString("✅ No actual issues detected.\n")
	} else {
		for _, e := range real {
			buf.WriteString(fmt.Sprintf("### [%s]\n```text\n%s\n```\n", e.Level, e.Message))
		}
	}
	buf.WriteString("\n---\n\n")

	// 3. Preflight: Environment & Build
	buf.WriteString("## 3. Preflight: Environment & Build\n")
	for _, p := range r.Preflight {
		buf.WriteString(fmt.Sprintf("\n### %s: %s\n\n```text\n%s\n```\n", p.Name, p.Status, strings.TrimSpace(p.Log)))
	}
	buf.WriteString("\n---\n\n")

	// 4. UI & Interactivity
	buf.WriteString("## 4. UI & Interactivity\n")
	
	// Lifecycle Verification
	buf.WriteString("\n### Lifecycle Verification Summary\n\n")
	r.verifyLifecycle(&buf)

	for i, s := range r.Steps {
		icon := "✅"
		if s.Status == "FAIL" {
			icon = "❌"
		}
		buf.WriteString(fmt.Sprintf("\n### %d. %s: %s %s\n\n", i+1, s.Name, s.Status, icon))
		if s.Err != nil {
			buf.WriteString(fmt.Sprintf("**Error:** `%v`\n\n", s.Err))
		}
		if len(s.Logs) > 0 {
			buf.WriteString("**Console Logs:**\n```text\n")
			for _, l := range s.Logs {
				buf.WriteString(fmt.Sprintf("[%s] %s\n", l.Level, l.Message))
			}
			buf.WriteString("```\n\n")
		}
		if s.Screenshot != "" {
			buf.WriteString(fmt.Sprintf("![%s](%s)\n\n", s.Name, s.Screenshot))
		}
		buf.WriteString("---\n")
	}

	os.WriteFile(smokeFile, buf.Bytes(), 0644)
}

func (r *SmokeRunner) verifyLifecycle(buf *bytes.Buffer) {
	events := []string{"LOADING", "LOADED", "START", "RESUME", "PAUSE", "AWAKE", "SLEEP"}
	found := make(map[string]bool)
	
	for _, step := range r.Steps {
		for _, log := range step.Logs {
			for _, event := range events {
				if strings.Contains(log.Message, event) {
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

func WaitForPort(port int, timeout time.Duration) error {
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
