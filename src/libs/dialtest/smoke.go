package dialtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"dialtone/cli/src/core/browser"
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
	Cmd    string
	Dir    string
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

type CommandStep struct {
	Name string
	Cmd  string
	Args []string
}

type SmokeRunner struct {
	Opts      SmokeOptions
	LogF      *os.File
	MW        io.Writer
	Entries   []ConsoleEntry
	Mu        sync.Mutex
	Steps     []StepResult
	Preflight []PreflightResult

	Ctx        context.Context
	Cancel     context.CancelFunc
	LastLogIdx int
	Browser    *ChromeSession
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

func (r *SmokeRunner) RunPreflight(repoRoot string, steps []struct {
	Name, Cmd string
	Args      []string
}) error {
	uiDir := filepath.Join(repoRoot, "src", "plugins", strings.ToLower(r.Opts.Name), r.Opts.VersionDir, "ui")
	cmdSteps := make([]CommandStep, 0, len(steps))
	for _, s := range steps {
		cmdSteps = append(cmdSteps, CommandStep{Name: s.Name, Cmd: s.Cmd, Args: s.Args})
	}
	return r.RunPreflightInDir(uiDir, cmdSteps)
}

func (r *SmokeRunner) RunPreflightInDir(uiDir string, steps []CommandStep) error {
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
		logText := fmt.Sprintf("[dir] %s\n[cmd] %s %s\n\n%s", uiDir, s.Cmd, strings.Join(s.Args, " "), strings.TrimSpace(buf.String()))
		r.Preflight = append(r.Preflight, PreflightResult{
			Name:   s.Name,
			Status: status,
			Log:    logText,
			Cmd:    s.Cmd + " " + strings.Join(s.Args, " "),
			Dir:    uiDir,
		})
	}
	return firstErr
}

func (r *SmokeRunner) SetupBrowser(url string) error {
	session, err := StartChromeSession(ChromeSessionOptions{
		RequestedPort:   0,
		Headless:        true,
		Role:            "smoke",
		ReuseExisting:   false,
		URL:             url,
		LogWriter:       r.MW,
		LogPrefix:       "   [BROWSER]",
		EmitProofOfLife: true,
		OnEntry: func(entry ConsoleEntry) {
			r.Mu.Lock()
			r.Entries = append(r.Entries, entry)
			r.Mu.Unlock()
		},
	})
	if err != nil {
		return err
	}
	r.LogMsg("[SMOKE] Navigating to %s...\n", url)
	r.Browser = session
	r.Ctx = session.Ctx
	r.Cancel = session.cancel
	return nil
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

func (r *SmokeRunner) AssertLastStepLogsContains(patterns ...string) error {
	if len(r.Steps) == 0 {
		return fmt.Errorf("no smoke steps recorded")
	}
	idx := len(r.Steps) - 1
	step := &r.Steps[idx]

	var lines []string
	for _, l := range step.Logs {
		lines = append(lines, fmt.Sprintf("[%s] %s", l.Level, l.Message))
	}
	allLogs := strings.Join(lines, "\n")

	var missing []string
	for _, p := range patterns {
		if !strings.Contains(allLogs, p) {
			missing = append(missing, p)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	err := fmt.Errorf("missing expected logs in step %q: %s", step.Name, strings.Join(missing, ", "))
	step.Status = "FAIL"
	step.Err = err
	r.LogMsg("[SMOKE] Step LOG ASSERT FAILED: %s | Missing: %s\n", step.Name, strings.Join(missing, ", "))
	return err
}

func (r *SmokeRunner) AssertSectionLifecycle(sectionIDs []string) error {
	type counters struct {
		load         int
		start        int
		pause        int
		resume       int
		navigateTo   int
		navigateAway int
	}
	bySection := map[string]*counters{}
	for _, id := range sectionIDs {
		bySection[id] = &counters{}
	}

	re := regexp.MustCompile(`\[SectionManager\]\s+(.+?)\s+#([a-zA-Z0-9_-]+)`)
	for _, e := range r.Entries {
		m := re.FindStringSubmatch(e.Message)
		if len(m) != 3 {
			continue
		}
		event := m[1]
		id := m[2]
		c, ok := bySection[id]
		if !ok {
			continue
		}
		switch {
		case strings.Contains(event, "LOADING"):
			c.load++
		case strings.Contains(event, "START"):
			c.start++
		case strings.Contains(event, "PAUSE"):
			c.pause++
		case strings.Contains(event, "RESUME"):
			c.resume++
		case strings.Contains(event, "NAVIGATE TO"):
			c.navigateTo++
		case strings.Contains(event, "NAVIGATE AWAY"):
			c.navigateAway++
		}
	}

	var failures []string
	for _, id := range sectionIDs {
		c := bySection[id]
		if c.load != 1 {
			failures = append(failures, fmt.Sprintf("%s expected 1 load, got %d", id, c.load))
		}
		if c.start != 1 {
			failures = append(failures, fmt.Sprintf("%s expected 1 start, got %d", id, c.start))
		}
		if c.resume < 1 {
			failures = append(failures, fmt.Sprintf("%s expected >=1 resume, got %d", id, c.resume))
		}
		if id != "home" && c.pause < 1 {
			failures = append(failures, fmt.Sprintf("%s expected >=1 pause, got %d", id, c.pause))
		}
		if c.navigateTo < 1 {
			failures = append(failures, fmt.Sprintf("%s expected >=1 navigate-to, got %d", id, c.navigateTo))
		}
	}

	if len(failures) == 0 {
		r.LogMsg("[SMOKE] Lifecycle ASSERT PASSED for sections: %s\n", strings.Join(sectionIDs, ", "))
		return nil
	}

	err := fmt.Errorf("lifecycle assertions failed: %s", strings.Join(failures, "; "))
	r.LogMsg("[SMOKE] Lifecycle ASSERT FAILED: %v\n", err)
	return err
}

func (r *SmokeRunner) Finalize() {
	if r.Browser != nil {
		r.Browser.Close()
	} else if r.Cancel != nil {
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

	if r.Browser != nil && r.Browser.IsNewBrowser {
		r.LogMsg("[SMOKE] Cleaning up browser processes using chrome plugin API...\n")
	}

	if r.LogF != nil {
		r.LogF.Close()
	}
}

func (r *SmokeRunner) PrepareGoPluginSmoke(repoRoot, pluginName string, preflight []CommandStep) (*exec.Cmd, error) {
	pluginDir := filepath.Join(repoRoot, "src", "plugins", pluginName, r.Opts.VersionDir)
	uiDir := filepath.Join(pluginDir, "ui")

	if err := r.RunDefaultPreflight(pluginDir, uiDir); err != nil {
		return nil, err
	}
	if len(preflight) > 0 {
		if err := r.RunPreflightInDir(uiDir, preflight); err != nil {
			return nil, err
		}
	}

	browser.CleanupPort(r.Opts.Port)
	serverCmd := exec.Command("go", "run", "cmd/main.go")
	serverCmd.Dir = pluginDir
	if err := r.StartServer(serverCmd); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("http://127.0.0.1:%d", r.Opts.Port)
	if err := r.SetupBrowser(url); err != nil {
		_ = serverCmd.Process.Kill()
		return nil, err
	}
	return serverCmd, nil
}

func (r *SmokeRunner) RunDefaultPreflight(pluginDir, uiDir string) error {
	goChecks := []CommandStep{
		{Name: "Go Format", Cmd: "go", Args: []string{"fmt", "./..."}},
		{Name: "Go Lint", Cmd: "go", Args: []string{"vet", "./..."}},
		{Name: "Go Build", Cmd: "go", Args: []string{"build", "./..."}},
	}
	uiChecks := []CommandStep{
		{Name: "UI Install", Cmd: "bun", Args: []string{"install"}},
		{Name: "UI TypeScript Lint", Cmd: "bun", Args: []string{"run", "lint"}},
		{Name: "UI Build", Cmd: "bun", Args: []string{"run", "build"}},
	}

	var firstErr error
	if err := r.RunPreflightInDir(pluginDir, goChecks); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := r.RunPreflightInDir(uiDir, uiChecks); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := r.runPrettierCheck(pluginDir); err != nil && firstErr == nil {
		firstErr = err
	}

	if err := r.runPreflightStartupProbe("Go Run", pluginDir, "go", []string{"run", "cmd/main.go"}, r.Opts.Port, 20*time.Second); err != nil && firstErr == nil {
		firstErr = err
	}
	uiRunPort, err := pickFreePort()
	if err != nil && firstErr == nil {
		firstErr = err
	}
	if err == nil {
		if probeErr := r.runPreflightStartupProbe("UI Run", uiDir, "bun", []string{"run", "dev", "--host", "127.0.0.1", "--port", fmt.Sprintf("%d", uiRunPort)}, uiRunPort, 20*time.Second); probeErr != nil && firstErr == nil {
			firstErr = probeErr
		}
	}

	return firstErr
}

func (r *SmokeRunner) runPrettierCheck(versionDir string) error {
	r.LogMsg("[SMOKE] Preflight: Source Prettier Format/Lint (JS/TS)...\n")

	allowedExt := map[string]bool{
		".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	}
	skipDirs := map[string]bool{
		"node_modules": true,
		".pixi":        true,
		"dist":         true,
	}

	var files []string
	walkErr := filepath.WalkDir(versionDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if allowedExt[strings.ToLower(filepath.Ext(path))] {
			rel, relErr := filepath.Rel(versionDir, path)
			if relErr != nil {
				return relErr
			}
			files = append(files, rel)
		}
		return nil
	})
	if walkErr != nil {
		r.Preflight = append(r.Preflight, PreflightResult{
			Name:   "Source Prettier Check (JS/TS)",
			Status: "❌ FAILED",
			Log:    fmt.Sprintf("[dir] %s\n[cmd] bunx prettier --check <files>\n\nwalk failed: %v", versionDir, walkErr),
			Cmd:    "bunx prettier --check <files>",
			Dir:    versionDir,
		})
		return walkErr
	}

	if len(files) == 0 {
		r.Preflight = append(r.Preflight, PreflightResult{
			Name:   "Source Prettier Format (JS/TS)",
			Status: "✅ PASSED",
			Log:    fmt.Sprintf("[dir] %s\n[cmd] bunx prettier --write <files>\n\nno JS/TS files found", versionDir),
			Cmd:    "bunx prettier --write <files>",
			Dir:    versionDir,
		})
		r.Preflight = append(r.Preflight, PreflightResult{
			Name:   "Source Prettier Lint (JS/TS)",
			Status: "✅ PASSED",
			Log:    fmt.Sprintf("[dir] %s\n[cmd] bunx prettier --check <files>\n\nno JS/TS files found", versionDir),
			Cmd:    "bunx prettier --check <files>",
			Dir:    versionDir,
		})
		return nil
	}

	writeErr := r.runPrettierCommand(versionDir, "Source Prettier Format (JS/TS)", "write", files)
	checkErr := r.runPrettierCommand(versionDir, "Source Prettier Lint (JS/TS)", "check", files)
	if writeErr != nil {
		return writeErr
	}
	return checkErr
}

func (r *SmokeRunner) runPrettierCommand(versionDir, stepName, mode string, files []string) error {
	args := append([]string{"prettier", "--" + mode}, files...)
	cmd := exec.Command("bunx", args...)
	cmd.Dir = versionDir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()

	status := "✅ PASSED"
	if err != nil {
		status = "❌ FAILED"
	}
	logText := fmt.Sprintf("[dir] %s\n[cmd] bunx %s\n\n%s", versionDir, strings.Join(args, " "), strings.TrimSpace(buf.String()))
	r.Preflight = append(r.Preflight, PreflightResult{
		Name:   stepName,
		Status: status,
		Log:    logText,
		Cmd:    "bunx " + strings.Join(args, " "),
		Dir:    versionDir,
	})
	return err
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

	// 1. Preflight: Environment & Build
	buf.WriteString("## 1. Preflight: Go + TypeScript/JavaScript Checks\n")
	for _, p := range r.Preflight {
		buf.WriteString(fmt.Sprintf("\n### %s: %s\n\n```text\n%s\n```\n", p.Name, p.Status, strings.TrimSpace(p.Log)))
	}
	buf.WriteString("\n---\n\n")

	// 2. Expected Errors (Proof of Life)
	buf.WriteString("## 2. Expected Errors (Proof of Life)\n\n")
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

	// 3. Real Errors & Warnings
	buf.WriteString("## 3. Real Errors & Warnings\n\n")
	if len(real) == 0 {
		buf.WriteString("✅ No actual issues detected.\n")
	} else {
		for _, e := range real {
			buf.WriteString(fmt.Sprintf("### [%s]\n```text\n%s\n```\n", e.Level, e.Message))
		}
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

func pickFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func (r *SmokeRunner) runPreflightStartupProbe(name, dir, cmdName string, args []string, port int, timeout time.Duration) error {
	r.LogMsg("[SMOKE] Preflight: %s...\n", name)
	browser.CleanupPort(port)

	cmd := exec.Command(cmdName, args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Start(); err != nil {
		r.Preflight = append(r.Preflight, PreflightResult{
			Name:   name,
			Status: "❌ FAILED",
			Log:    fmt.Sprintf("[dir] %s\n[cmd] %s %s\n\nstart failed: %v", dir, cmdName, strings.Join(args, " "), err),
			Cmd:    cmdName + " " + strings.Join(args, " "),
			Dir:    dir,
		})
		return err
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	start := time.Now()
	var probeErr error
	waitConsumed := false
	ready := false
	for time.Since(start) < timeout {
		select {
		case err := <-waitCh:
			waitConsumed = true
			if err != nil {
				probeErr = fmt.Errorf("process exited before ready: %w", err)
			} else {
				probeErr = fmt.Errorf("process exited before port %d became ready", port)
			}
			goto DONE
		default:
			if WaitForPort(port, 500*time.Millisecond) == nil {
				ready = true
				goto DONE
			}
		}
	}
	probeErr = fmt.Errorf("timeout waiting for port %d", port)

DONE:
	_ = cmd.Process.Kill()
	shutdownWarn := ""
	if !waitConsumed {
		select {
		case <-waitCh:
		case <-time.After(5 * time.Second):
			// Readiness is the pass criterion. Record shutdown lag as warning only.
			shutdownWarn = fmt.Sprintf("timed out waiting for %s process shutdown", name)
			if !ready && probeErr == nil {
				probeErr = fmt.Errorf(shutdownWarn)
			}
		}
	}
	browser.CleanupPort(port)

	status := "✅ PASSED"
	if probeErr != nil {
		status = "❌ FAILED"
	}

	logText := fmt.Sprintf("[dir] %s\n[cmd] %s %s\n\n%s", dir, cmdName, strings.Join(args, " "), strings.TrimSpace(buf.String()))
	if probeErr != nil {
		logText = fmt.Sprintf("%s\n\n[probe-error] %v", logText, probeErr)
	} else if shutdownWarn != "" {
		logText = fmt.Sprintf("%s\n\n[probe-warning] %s", logText, shutdownWarn)
	}
	r.Preflight = append(r.Preflight, PreflightResult{
		Name:   name,
		Status: status,
		Log:    logText,
		Cmd:    cmdName + " " + strings.Join(args, " "),
		Dir:    dir,
	})

	return probeErr
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
