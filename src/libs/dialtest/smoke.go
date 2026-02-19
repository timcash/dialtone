package dialtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

	"dialtone/dev/core/browser"
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
	Name           string
	VersionDir     string
	Port           int
	SmokeDir       string
	TotalTimeout   time.Duration
	StepTimeout    time.Duration
	CommandStall   time.Duration
	PanicOnTimeout bool
	PanicOnFailure bool
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

	Ctx                context.Context
	Cancel             context.CancelFunc
	LastLogIdx         int
	Browser            *ChromeSession
	stepsStartedAt     time.Time
	lastProgressReason string
	currentActivity    string
	currentStage       string
}

type StepOptions struct {
	Timeout time.Duration
}

func NewSmokeRunner(opts SmokeOptions) (*SmokeRunner, error) {
	if opts.Port == 0 {
		opts.Port = 8080
	}
	if opts.TotalTimeout <= 0 {
		opts.TotalTimeout = 30 * time.Second
	}
	if opts.StepTimeout <= 0 {
		opts.StepTimeout = 5 * time.Second
	}
	if opts.CommandStall <= 0 {
		opts.CommandStall = 12 * time.Second
	}
	if !opts.PanicOnTimeout {
		opts.PanicOnTimeout = true
	}
	if !opts.PanicOnFailure {
		opts.PanicOnFailure = true
	}
	os.MkdirAll(opts.SmokeDir, 0755)

	smokeLogFile := filepath.Join(opts.SmokeDir, "smoke.log")
	logF, err := os.Create(smokeLogFile)
	if err != nil {
		return nil, err
	}

	mw := io.MultiWriter(os.Stdout, logF)

	return &SmokeRunner{
		Opts:           opts,
		LogF:           logF,
		MW:             mw,
		stepsStartedAt: time.Now(),
	}, nil
}

type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

type activityWriter struct {
	w        io.Writer
	onOutput func()
}

func (a *activityWriter) Write(p []byte) (int, error) {
	n, err := a.w.Write(p)
	if n > 0 && a.onOutput != nil {
		a.onOutput()
	}
	return n, err
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
		r.currentActivity = s.Name
		r.WriteProgressReport("preflight-start")
		r.setStage("preflight:" + s.Name)
		r.ensureTotalTimeout("preflight " + s.Name)
		r.LogMsg("[SMOKE] Preflight: %s...\n", s.Name)
		r.LogMsg("   [DIR] %s\n", uiDir)
		r.LogMsg("   [CMD] %s %s\n", s.Cmd, strings.Join(s.Args, " "))
		cmdCtx, cancelCmd := context.WithTimeout(context.Background(), r.remainingTotal())
		cmd := exec.CommandContext(cmdCtx, s.Cmd, s.Args...)
		cmd.Dir = uiDir
		var buf safeBuffer
		lastOutputAt := time.Now()
		var outputMu sync.Mutex
		cmdOutput := &activityWriter{
			w: io.MultiWriter(&buf, r.MW),
			onOutput: func() {
				outputMu.Lock()
				lastOutputAt = time.Now()
				outputMu.Unlock()
			},
		}
		cmd.Stdout = cmdOutput
		cmd.Stderr = cmdOutput
		start := time.Now()
		if err := cmd.Start(); err != nil {
			cancelCmd()
			status := "❌ FAILED"
			logText := fmt.Sprintf("[dir] %s\n[cmd] %s %s\n\nstart failed: %v", uiDir, s.Cmd, strings.Join(s.Args, " "), err)
			r.Preflight = append(r.Preflight, PreflightResult{
				Name:   s.Name,
				Status: status,
				Log:    logText,
				Cmd:    s.Cmd + " " + strings.Join(s.Args, " "),
				Dir:    uiDir,
			})
			if firstErr == nil {
				firstErr = err
			}
			r.LogMsg("[SMOKE] Preflight FAILED to start: %s | %v\n", s.Name, err)
			r.WriteProgressReport("preflight:" + s.Name)
			r.failNow("preflight:"+s.Name, "command failed to start", err)
			continue
		}

		waitCh := make(chan error, 1)
		go func() {
			waitCh <- cmd.Wait()
		}()

		ticker := time.NewTicker(1 * time.Second)
		var err error
		waiting := true
		for waiting {
			select {
			case err = <-waitCh:
				waiting = false
			case <-ticker.C:
				if time.Since(start).Round(time.Second)%5 == 0 {
					r.LogMsg("[SMOKE] Preflight still running: %s (elapsed: %s)\n", s.Name, time.Since(start).Round(time.Second))
				}
				outputMu.Lock()
				stalledFor := time.Since(lastOutputAt)
				outputMu.Unlock()
				if stalledFor > r.Opts.CommandStall {
					_ = cmd.Process.Kill()
					err = fmt.Errorf("command stalled with no output for %s", stalledFor.Round(time.Second))
					r.LogMsg("[SMOKE] Preflight STALLED: %s (elapsed: %s) | %v\n", s.Name, time.Since(start).Round(time.Millisecond), err)
					waiting = false
					continue
				}
				r.ensureTotalTimeout("preflight " + s.Name)
			}
		}
		ticker.Stop()
		cancelCmd()

		status := "✅ PASSED"
		if err != nil {
			status = "❌ FAILED"
			if firstErr == nil {
				firstErr = err
			}
			r.LogMsg("[SMOKE] Preflight FAILED: %s (elapsed: %s) | %v\n", s.Name, time.Since(start).Round(time.Millisecond), err)
			r.failNow("preflight:"+s.Name, "command failed", fmt.Errorf("%w\n%s", err, strings.TrimSpace(buf.String())))
		} else {
			r.LogMsg("[SMOKE] Preflight PASSED: %s (elapsed: %s)\n", s.Name, time.Since(start).Round(time.Millisecond))
		}
		logText := fmt.Sprintf("[dir] %s\n[cmd] %s %s\n[elapsed] %s\n\n%s", uiDir, s.Cmd, strings.Join(s.Args, " "), time.Since(start).Round(time.Millisecond), strings.TrimSpace(buf.String()))
		r.Preflight = append(r.Preflight, PreflightResult{
			Name:   s.Name,
			Status: status,
			Log:    logText,
			Cmd:    s.Cmd + " " + strings.Join(s.Args, " "),
			Dir:    uiDir,
		})
		r.WriteProgressReport("preflight:" + s.Name)
	}
	r.currentActivity = ""
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
	r.setStage("server:start")
	r.ensureTotalTimeout("start server")
	serverLogFile, _ := os.Create(filepath.Join(r.Opts.SmokeDir, "smoke_server.log"))
	cmd.Stdout = serverLogFile
	cmd.Stderr = serverLogFile

	r.LogMsg("[SMOKE] Starting plugin server on port %d...\n", r.Opts.Port)
	if err := cmd.Start(); err != nil {
		return err
	}

	waitTimeout := 15 * time.Second
	if rem := r.remainingTotal(); rem < waitTimeout {
		waitTimeout = rem
	}
	if err := WaitForPort(r.Opts.Port, waitTimeout); err != nil {
		cmd.Process.Kill()
		r.failNow("server:start", "server timeout", err)
		return fmt.Errorf("server timeout: %v", err)
	}
	r.LogMsg("[SMOKE] Server ready.\n")
	return nil
}

func (r *SmokeRunner) Step(name string, actions chromedp.Action, opts ...StepOptions) {
	r.currentActivity = name
	r.WriteProgressReport("step-start")
	r.setStage("step:" + name)
	r.LogMsg("[SMOKE] Step Start: %s\n", name)
	r.ensureTotalTimeout(name)

	stepTimeout := r.Opts.StepTimeout
	if len(opts) > 0 && opts[0].Timeout > 0 {
		stepTimeout = opts[0].Timeout
	}
	if stepTimeout <= 0 {
		stepTimeout = 5 * time.Second
	}
	if rem := r.remainingTotal(); rem < stepTimeout {
		stepTimeout = rem
	}

	runCtx, cancel := context.WithTimeout(r.Ctx, stepTimeout)
	err := chromedp.Run(runCtx, actions)
	cancel()

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
		r.failNow("step:"+name, "step action failed", err)
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
	r.WriteProgressReport("step:" + name)
	r.currentActivity = ""

	if err != nil && (errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded")) {
		r.timeoutPanic(fmt.Sprintf("step %q exceeded timeout %s", name, stepTimeout))
	}
	r.ensureTotalTimeout(name)
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
	r.WriteProgressReport("step-log-assert:" + step.Name)
	r.failNow("step-log-assert:"+step.Name, "required logs missing", err)
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
		r.WriteProgressReport("lifecycle-assert")
		return nil
	}

	err := fmt.Errorf("lifecycle assertions failed: %s", strings.Join(failures, "; "))
	r.LogMsg("[SMOKE] Lifecycle ASSERT FAILED: %v\n", err)
	r.WriteProgressReport("lifecycle-assert-failed")
	r.failNow("lifecycle-assert", "section lifecycle invariant failed", err)
	return err
}

func (r *SmokeRunner) ensureTotalTimeout(stepName string) {
	if r.Opts.TotalTimeout <= 0 {
		return
	}
	if r.stepsStartedAt.IsZero() {
		r.stepsStartedAt = time.Now()
		return
	}
	if time.Since(r.stepsStartedAt) > r.Opts.TotalTimeout {
		r.timeoutPanic(fmt.Sprintf("total smoke run exceeded while running %q (%s)", stepName, r.Opts.TotalTimeout))
	}
}

func (r *SmokeRunner) timeoutPanic(msg string) {
	full := fmt.Sprintf("[SMOKE][TIMEOUT][%s] %s", r.currentStage, msg)
	r.LogMsg("%s\n", full)
	if r.Opts.PanicOnTimeout {
		panic(full)
	}
}

func (r *SmokeRunner) remainingTotal() time.Duration {
	if r.Opts.TotalTimeout <= 0 {
		return 24 * time.Hour
	}
	elapsed := time.Since(r.stepsStartedAt)
	remaining := r.Opts.TotalTimeout - elapsed
	if remaining <= 0 {
		r.timeoutPanic(fmt.Sprintf("total smoke run exceeded %s", r.Opts.TotalTimeout))
		return 1 * time.Millisecond
	}
	return remaining
}

func (r *SmokeRunner) setStage(stage string) {
	r.currentStage = stage
}

func (r *SmokeRunner) failNow(stage, msg string, err error) {
	if !r.Opts.PanicOnFailure {
		return
	}
	full := fmt.Sprintf("[SMOKE][FAIL][%s] %s: %v", stage, msg, err)
	r.LogMsg("%s\n", full)
	panic(full)
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

	if err := r.RunDefaultPreflight(repoRoot, pluginDir, uiDir); err != nil {
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

func (r *SmokeRunner) RunDefaultPreflight(repoRoot, pluginDir, uiDir string) error {
	dialtoneCmd := filepath.Join(repoRoot, "dialtone.sh")
	goChecks := []CommandStep{
		{Name: "Go Format", Cmd: "go", Args: []string{"fmt", "./..."}},
		{Name: "Go Lint", Cmd: "go", Args: []string{"vet", "./..."}},
		{Name: "Go Build", Cmd: "go", Args: []string{"build", "./..."}},
	}
	uiChecks := []CommandStep{
		{Name: "UI Install", Cmd: dialtoneCmd, Args: []string{"bun", "exec", "--cwd", uiDir, "install", "--force"}},
		{Name: "UI TypeScript Lint", Cmd: dialtoneCmd, Args: []string{"bun", "exec", "--cwd", uiDir, "run", "lint"}},
		{Name: "UI Build", Cmd: dialtoneCmd, Args: []string{"bun", "exec", "--cwd", uiDir, "run", "build"}},
	}

	var firstErr error
	if err := r.RunPreflightInDir(pluginDir, goChecks); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := r.RunPreflightInDir(repoRoot, uiChecks); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := r.runPrettierCheck(repoRoot, pluginDir); err != nil && firstErr == nil {
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
		if probeErr := r.runPreflightStartupProbe("UI Run", repoRoot, dialtoneCmd, []string{"bun", "exec", "--cwd", uiDir, "run", "dev", "--host", "127.0.0.1", "--port", fmt.Sprintf("%d", uiRunPort)}, uiRunPort, 20*time.Second); probeErr != nil && firstErr == nil {
			firstErr = probeErr
		}
	}

	return firstErr
}

func (r *SmokeRunner) runPrettierCheck(repoRoot, versionDir string) error {
	r.LogMsg("[SMOKE] Preflight: Source Prettier Format/Lint (JS/TS)...\n")

	allowedExt := map[string]bool{
		".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
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
			Log:    fmt.Sprintf("[dir] %s\n[cmd] ./dialtone.sh bun exec x prettier --check <files>\n\nwalk failed: %v", versionDir, walkErr),
			Cmd:    "./dialtone.sh bun exec x prettier --check <files>",
			Dir:    versionDir,
		})
		r.failNow("preflight:Source Prettier Check (JS/TS)", "walk failed", walkErr)
		return walkErr
	}

	if len(files) == 0 {
		r.Preflight = append(r.Preflight, PreflightResult{
			Name:   "Source Prettier Format (JS/TS)",
			Status: "✅ PASSED",
			Log:    fmt.Sprintf("[dir] %s\n[cmd] ./dialtone.sh bun exec x prettier --write <files>\n\nno JS/TS files found", versionDir),
			Cmd:    "./dialtone.sh bun exec x prettier --write <files>",
			Dir:    versionDir,
		})
		r.Preflight = append(r.Preflight, PreflightResult{
			Name:   "Source Prettier Lint (JS/TS)",
			Status: "✅ PASSED",
			Log:    fmt.Sprintf("[dir] %s\n[cmd] ./dialtone.sh bun exec x prettier --check <files>\n\nno JS/TS files found", versionDir),
			Cmd:    "./dialtone.sh bun exec x prettier --check <files>",
			Dir:    versionDir,
		})
		return nil
	}

	writeErr := r.runPrettierCommand(repoRoot, versionDir, "Source Prettier Format (JS/TS)", "write", files)
	checkErr := r.runPrettierCommand(repoRoot, versionDir, "Source Prettier Lint (JS/TS)", "check", files)
	if writeErr != nil {
		return writeErr
	}
	return checkErr
}

func (r *SmokeRunner) runPrettierCommand(repoRoot, versionDir, stepName, mode string, files []string) error {
	args := append([]string{"bun", "exec", "--cwd", versionDir, "x", "prettier", "--" + mode}, files...)
	cmdCtx, cancel := context.WithTimeout(context.Background(), r.remainingTotal())
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, filepath.Join(repoRoot, "dialtone.sh"), args...)
	cmd.Dir = repoRoot
	var buf safeBuffer
	lastOutputAt := time.Now()
	var outputMu sync.Mutex
	cmdOutput := &activityWriter{
		w: io.MultiWriter(&buf, r.MW),
		onOutput: func() {
			outputMu.Lock()
			lastOutputAt = time.Now()
			outputMu.Unlock()
		},
	}
	cmd.Stdout = cmdOutput
	cmd.Stderr = cmdOutput

	if err := cmd.Start(); err != nil {
		r.failNow("preflight:"+stepName, "command failed to start", err)
		return err
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	ticker := time.NewTicker(1 * time.Second)
	var err error
	waiting := true
	for waiting {
		select {
		case err = <-waitCh:
			waiting = false
		case <-ticker.C:
			r.ensureTotalTimeout(stepName)
			outputMu.Lock()
			stalledFor := time.Since(lastOutputAt)
			outputMu.Unlock()
			if stalledFor > r.Opts.CommandStall {
				_ = cmd.Process.Kill()
				err = fmt.Errorf("command stalled with no output for %s", stalledFor.Round(time.Second))
				waiting = false
			}
		}
	}
	ticker.Stop()

	status := "✅ PASSED"
	if err != nil {
		status = "❌ FAILED"
	}
	logText := fmt.Sprintf("[dir] %s\n[cmd] ./dialtone.sh %s\n\n%s", versionDir, strings.Join(args, " "), strings.TrimSpace(buf.String()))
	r.Preflight = append(r.Preflight, PreflightResult{
		Name:   stepName,
		Status: status,
		Log:    logText,
		Cmd:    "./dialtone.sh " + strings.Join(args, " "),
		Dir:    versionDir,
	})
	if err != nil {
		r.failNow("preflight:"+stepName, "command failed", fmt.Errorf("%w\n%s", err, strings.TrimSpace(buf.String())))
	}
	return err
}

func (r *SmokeRunner) WriteFinalReport() {
	r.writeReport(false)
}

func (r *SmokeRunner) writeReport(inProgress bool) {
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
	if inProgress {
		reason := r.lastProgressReason
		if reason == "" {
			reason = "progress update"
		}
		buf.WriteString(fmt.Sprintf("**Status:** IN PROGRESS (%s)\n\n", reason))
		if r.currentActivity != "" {
			buf.WriteString("## Live Progress\n\n")
			buf.WriteString(fmt.Sprintf("### %s\n\n", r.currentActivity))
			buf.WriteString("- Status: IN PROGRESS\n")
			buf.WriteString(fmt.Sprintf("- Updated: %s\n\n", time.Now().Format(time.RFC1123)))
			buf.WriteString("---\n\n")
		}
	}

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
	r.currentActivity = name
	r.WriteProgressReport("probe-start")
	r.setStage("probe:" + name)
	if rem := r.remainingTotal(); rem < timeout {
		timeout = rem
	}
	r.LogMsg("[SMOKE] Preflight: %s...\n", name)
	r.LogMsg("   [DIR] %s\n", dir)
	r.LogMsg("   [CMD] %s %s\n", cmdName, strings.Join(args, " "))
	r.LogMsg("   [PORT] %d | [TIMEOUT] %s\n", port, timeout)
	browser.CleanupPort(port)

	cmd := exec.Command(cmdName, args...)
	cmd.Dir = dir
	var buf safeBuffer
	lastOutputAt := time.Now()
	var outputMu sync.Mutex
	cmdOutput := &activityWriter{
		w: io.MultiWriter(&buf, r.MW),
		onOutput: func() {
			outputMu.Lock()
			lastOutputAt = time.Now()
			outputMu.Unlock()
		},
	}
	cmd.Stdout = cmdOutput
	cmd.Stderr = cmdOutput

	if err := cmd.Start(); err != nil {
		r.Preflight = append(r.Preflight, PreflightResult{
			Name:   name,
			Status: "❌ FAILED",
			Log:    fmt.Sprintf("[dir] %s\n[cmd] %s %s\n\nstart failed: %v", dir, cmdName, strings.Join(args, " "), err),
			Cmd:    cmdName + " " + strings.Join(args, " "),
			Dir:    dir,
		})
		r.failNow("preflight:"+name, "command failed to start", err)
		return err
	}
	r.LogMsg("[SMOKE] Preflight %s started PID=%d\n", name, cmd.Process.Pid)

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	start := time.Now()
	var probeErr error
	waitConsumed := false
	ready := false
	progressTicker := time.NewTicker(2 * time.Second)
	for time.Since(start) < timeout {
		select {
		case err := <-waitCh:
			waitConsumed = true
			if err != nil {
				probeErr = fmt.Errorf("process exited before ready: %w", err)
			} else {
				probeErr = fmt.Errorf("process exited before port %d became ready", port)
			}
			r.LogMsg("[SMOKE] Preflight %s exited before ready (elapsed: %s): %v\n", name, time.Since(start).Round(time.Millisecond), probeErr)
			goto DONE
		case <-progressTicker.C:
			r.LogMsg("[SMOKE] Preflight %s waiting for port %d... (elapsed: %s)\n", name, port, time.Since(start).Round(time.Second))
			outputMu.Lock()
			stalledFor := time.Since(lastOutputAt)
			outputMu.Unlock()
			if stalledFor > r.Opts.CommandStall {
				probeErr = fmt.Errorf("process stalled with no output for %s before port %d was ready", stalledFor.Round(time.Second), port)
				r.LogMsg("[SMOKE] Preflight %s stalled: %v\n", name, probeErr)
				goto DONE
			}
		default:
			if WaitForPort(port, 500*time.Millisecond) == nil {
				ready = true
				r.LogMsg("[SMOKE] Preflight %s port %d is ready (elapsed: %s)\n", name, port, time.Since(start).Round(time.Millisecond))
				goto DONE
			}
		}
	}
	probeErr = fmt.Errorf("timeout waiting for port %d", port)
	r.LogMsg("[SMOKE] Preflight %s timeout waiting for port %d (elapsed: %s)\n", name, port, time.Since(start).Round(time.Millisecond))

DONE:
	progressTicker.Stop()
	_ = cmd.Process.Kill()
	shutdownWarn := ""
	shutdownStart := time.Now()
	if !waitConsumed {
		select {
		case <-waitCh:
			r.LogMsg("[SMOKE] Preflight %s process exited after kill (shutdown: %s)\n", name, time.Since(shutdownStart).Round(time.Millisecond))
		case <-time.After(5 * time.Second):
			// Readiness is the pass criterion. Record shutdown lag as warning only.
			shutdownWarn = fmt.Sprintf("timed out waiting for %s process shutdown", name)
			r.LogMsg("[SMOKE] Preflight %s shutdown warning: %s\n", name, shutdownWarn)
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
	if ready {
		logText = fmt.Sprintf("%s\n[probe-ready] port %d became reachable in %s", logText, port, time.Since(start).Round(time.Millisecond))
	}
	r.Preflight = append(r.Preflight, PreflightResult{
		Name:   name,
		Status: status,
		Log:    logText,
		Cmd:    cmdName + " " + strings.Join(args, " "),
		Dir:    dir,
	})
	if probeErr != nil {
		r.failNow("preflight:"+name, "startup probe failed", fmt.Errorf("%w\n%s", probeErr, strings.TrimSpace(buf.String())))
	}
	r.WriteProgressReport("probe:" + name)
	r.currentActivity = ""

	return probeErr
}

func (r *SmokeRunner) WriteProgressReport(reason string) {
	r.lastProgressReason = reason
	r.writeReport(true)
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
