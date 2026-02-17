package test_v2

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	chrome_app "dialtone/cli/src/plugins/chrome/app"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

type ConsoleEntry struct {
	Level   string
	Message string
	At      time.Time
}

type BrowserOptions struct {
	Headless        bool
	Role            string
	ReuseExisting   bool
	URL             string
	LogWriter       io.Writer
	LogPrefix       string
	EmitProofOfLife bool
}

type BrowserSession struct {
	ctx       context.Context
	cancel    context.CancelFunc
	isNew     bool
	chromeSes *chrome_app.Session

	mu      sync.Mutex
	entries []ConsoleEntry
}

var (
	stepLogsMu sync.Mutex
	stepLogs   []ConsoleEntry
	stepLogsOn bool
)

func startStepLogCapture() {
	stepLogsMu.Lock()
	defer stepLogsMu.Unlock()
	stepLogs = stepLogs[:0]
	stepLogsOn = true
}

func appendStepLog(entry ConsoleEntry) {
	stepLogsMu.Lock()
	defer stepLogsMu.Unlock()
	if !stepLogsOn {
		return
	}
	stepLogs = append(stepLogs, entry)
}

func endStepLogCapture() []ConsoleEntry {
	stepLogsMu.Lock()
	defer stepLogsMu.Unlock()
	out := make([]ConsoleEntry, len(stepLogs))
	copy(out, stepLogs)
	stepLogsOn = false
	return out
}

type lockedBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

type elapsedPrefixWriter struct {
	mu      sync.Mutex
	started time.Time
	dst     io.Writer
	pending string
}

func (w *elapsedPrefixWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.pending += string(p)
	for {
		idx := strings.IndexByte(w.pending, '\n')
		if idx < 0 {
			break
		}
		line := w.pending[:idx]
		w.pending = w.pending[idx+1:]
		if err := w.writeLine(line); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (w *elapsedPrefixWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pending == "" {
		return nil
	}
	line := w.pending
	w.pending = ""
	return w.writeLine(line)
}

func (w *elapsedPrefixWriter) writeLine(line string) error {
	_, err := fmt.Fprintf(w.dst, "%s %s\n", elapsedTag(w.started, time.Now()), line)
	return err
}

func elapsedTag(started time.Time, at time.Time) string {
	secs := int(at.Sub(started).Seconds())
	if secs < 0 {
		secs = 0
	}
	return fmt.Sprintf("[T+%04d]", secs)
}

func captureStepOutput(started time.Time, run func() error) (string, error) {
	origStdout := os.Stdout
	origStderr := os.Stderr

	rOut, wOut, err := os.Pipe()
	if err != nil {
		return "", err
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		_ = rOut.Close()
		_ = wOut.Close()
		return "", err
	}

	os.Stdout = wOut
	os.Stderr = wErr

	var buf lockedBuffer
	prefixed := &elapsedPrefixWriter{
		started: started,
		dst:     io.MultiWriter(origStdout, &buf),
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(prefixed, rOut)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(prefixed, rErr)
	}()

	runErr := run()

	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr
	_ = rOut.Close()
	_ = rErr.Close()
	wg.Wait()
	_ = prefixed.Flush()

	return buf.String(), runErr
}

func StartBrowser(opts BrowserOptions) (*BrowserSession, error) {
	s := &BrowserSession{}
	if opts.Role == "" {
		opts.Role = "default"
	}

	resolved, err := chrome_app.StartSession(chrome_app.SessionOptions{
		GPU:           true,
		Headless:      opts.Headless,
		TargetURL:     opts.URL,
		Role:          opts.Role,
		ReuseExisting: opts.ReuseExisting,
	})
	if err != nil {
		return nil, err
	}

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), resolved.WebSocketURL)
	ctx, ctxCancel, err := attachOrCreateTargetContext(allocCtx, resolved.WebSocketURL, resolved.IsNew, opts.ReuseExisting, opts.URL)
	if err != nil {
		allocCancel()
		return nil, err
	}
	cancel := func() {
		ctxCancel()
		allocCancel()
	}

	logPrefix := opts.LogPrefix
	if logPrefix == "" {
		logPrefix = "[BROWSER]"
	}

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		var entry *ConsoleEntry
		switch e := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			entry = &ConsoleEntry{Level: string(e.Type), Message: formatConsoleArgs(e.Args), At: time.Now()}
		case *runtime.EventExceptionThrown:
			msg := e.ExceptionDetails.Text
			if e.ExceptionDetails.Exception != nil && e.ExceptionDetails.Exception.Description != "" {
				msg = e.ExceptionDetails.Exception.Description
			}
			entry = &ConsoleEntry{Level: "exception", Message: msg, At: time.Now()}
		}
		if entry == nil {
			return
		}
		s.mu.Lock()
		s.entries = append(s.entries, *entry)
		s.mu.Unlock()
		appendStepLog(*entry)
		if opts.LogWriter != nil {
			fmt.Fprintf(opts.LogWriter, "%s [%s] %s\n", logPrefix, entry.Level, entry.Message)
		}
	})

	tasks := chromedp.Tasks{}
	if opts.URL != "" {
		tasks = append(tasks,
			chromedp.EmulateViewport(1280, 800),
			chromedp.Navigate(opts.URL),
		)
	}
	if opts.EmitProofOfLife {
		tasks = append(tasks, chromedp.Evaluate(`console.error('[PROOFOFLIFE] Intentional Browser Test Error')`, nil))
	}
	if len(tasks) > 0 {
		if err := chromedp.Run(ctx, tasks); err != nil {
			cancel()
			return nil, err
		}
	}

	s.ctx = ctx
	s.cancel = cancel
	s.isNew = resolved.IsNew
	s.chromeSes = resolved
	return s, nil
}

func attachOrCreateTargetContext(allocCtx context.Context, websocketURL string, isNew, reuse bool, targetURL string) (context.Context, context.CancelFunc, error) {
	_ = isNew
	if strings.TrimSpace(targetURL) != "" {
		targetID, err := selectTargetIDFromWebsocket(websocketURL, targetURL)
		if err != nil {
			return nil, nil, err
		}
		ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithTargetID(target.ID(targetID)))
		return ctx, cancel, nil
	}
	if reuse {
		targetID, err := selectTargetIDFromWebsocket(websocketURL, "")
		if err == nil {
			ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithTargetID(target.ID(targetID)))
			return ctx, cancel, nil
		}
	}
	ctx, cancel := chromedp.NewContext(allocCtx)
	return ctx, cancel, nil
}

type devtoolsTarget struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

func selectTargetIDFromWebsocket(websocketURL, preferredURL string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(websocketURL))
	if err != nil {
		return "", fmt.Errorf("invalid websocket url: %w", err)
	}
	scheme := "http"
	if base.Scheme == "wss" {
		scheme = "https"
	}
	listURL := fmt.Sprintf("%s://%s/json/list", scheme, base.Host)
	client := &http.Client{Timeout: 2 * time.Second}
	want := normalizeURLWithoutFragment(preferredURL)

	for i := 0; i < 30; i++ {
		resp, err := client.Get(listURL)
		if err == nil {
			var infos []devtoolsTarget
			decodeErr := json.NewDecoder(resp.Body).Decode(&infos)
			_ = resp.Body.Close()
			if decodeErr == nil {
				bestID := ""
				bestScore := -1
				for _, info := range infos {
					if info.Type != "page" {
						continue
					}
					u := strings.TrimSpace(info.URL)
					if strings.HasPrefix(u, "devtools://") {
						continue
					}
					score := 1
					if u == "" || u == "about:blank" {
						score = 0
					}
					if want != "" && normalizeURLWithoutFragment(u) == want {
						score = 100
					}
					if score > bestScore {
						bestScore = score
						bestID = info.ID
					}
				}
				if bestID != "" {
					return bestID, nil
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return "", fmt.Errorf("failed to attach to an existing browser page target")
}

func normalizeURLWithoutFragment(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	parsed.Fragment = ""
	return parsed.String()
}

func (s *BrowserSession) Close() {
	if s == nil {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.isNew {
		_ = chrome_app.CleanupSession(s.chromeSes)
	}
}

func (s *BrowserSession) Entries() []ConsoleEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ConsoleEntry, len(s.entries))
	copy(out, s.entries)
	return out
}

func (s *BrowserSession) HasConsoleMessage(substr string) bool {
	for _, e := range s.Entries() {
		if strings.Contains(e.Message, substr) {
			return true
		}
	}
	return false
}

func (s *BrowserSession) Run(actions chromedp.Action) error {
	if s == nil || s.ctx == nil {
		return fmt.Errorf("browser session is not initialized")
	}
	return chromedp.Run(s.ctx, actions)
}

func (s *BrowserSession) RunWithContext(ctx context.Context, actions chromedp.Action) error {
	if s == nil || s.ctx == nil {
		return fmt.Errorf("browser session is not initialized")
	}
	return chromedp.Run(ctx, actions)
}

func (s *BrowserSession) Context() context.Context {
	return s.ctx
}

func (s *BrowserSession) ChromeSession() *chrome_app.Session {
	if s == nil {
		return nil
	}
	return s.chromeSes
}

func (s *BrowserSession) CaptureScreenshot(path string) error {
	if s == nil || s.ctx == nil {
		return fmt.Errorf("browser session is not initialized")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	var shot []byte
	if err := chromedp.Run(s.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		b, err := page.CaptureScreenshot().Do(ctx)
		if err != nil {
			return err
		}
		shot = b
		return nil
	})); err != nil {
		return err
	}
	if len(shot) == 0 {
		return fmt.Errorf("empty screenshot")
	}
	return os.WriteFile(path, shot, 0o644)
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
				switch vv := v.(type) {
				case string:
					parts = append(parts, vv)
				default:
					b, _ := json.Marshal(v)
					parts = append(parts, string(b))
				}
			} else {
				parts = append(parts, strings.Trim(string(arg.Value), `"`))
			}
		} else if arg.Description != "" {
			parts = append(parts, arg.Description)
		}
	}
	return strings.Join(parts, " ")
}

func filterLogsForStep(logs []ConsoleEntry, sectionID string, mode string) []ConsoleEntry {
	filtered := logs
	if sectionID != "" {
		token := "#" + sectionID
		out := make([]ConsoleEntry, 0, len(filtered))
		for _, entry := range filtered {
			if strings.Contains(entry.Message, token) {
				out = append(out, entry)
			}
		}
		filtered = out
	}
	switch mode {
	case "errors_only":
		out := make([]ConsoleEntry, 0, len(filtered))
		for _, entry := range filtered {
			if entry.Level == "error" || entry.Level == "exception" {
				out = append(out, entry)
			}
		}
		return out
	case "test_tagged":
		out := make([]ConsoleEntry, 0, len(filtered))
		for _, entry := range filtered {
			if entry.Level == "error" || entry.Level == "exception" || strings.Contains(entry.Message, "[TESTLIB]") {
				out = append(out, entry)
			}
		}
		return out
	default:
		return filtered
	}
}

type Step struct {
	Name           string
	Run            func() error
	RunWithContext func(*StepContext) (StepRunResult, error)
	SectionID      string
	Screenshot     string
	Screenshots    []string
	ScreenshotGrid string
	Timeout        time.Duration
}

type StepContext struct {
	Name    string
	Started time.Time
}

type StepRunResult struct {
	Report string
}

type SuiteOptions struct {
	Version        string
	ReportPath     string
	LogPath        string
	ErrorLogPath   string
	BrowserLogMode string
}

type StepResult struct {
	Name           string
	Passed         bool
	Error          string
	Duration       time.Duration
	SectionID      string
	Screenshot     string
	Screenshots    []string
	ScreenshotGrid string
	Logs           []ConsoleEntry
	Output         string
	Report         string
}

func RunSuite(options SuiteOptions, steps []Step) error {
	logFile, err := os.Create(options.LogPath)
	if err != nil {
		return err
	}
	defer logFile.Close()

	errorLogPath := options.ErrorLogPath
	if strings.TrimSpace(errorLogPath) == "" {
		errorLogPath = filepath.Join(filepath.Dir(options.LogPath), "error.log")
	}
	errorFile, err := os.Create(errorLogPath)
	if err != nil {
		return err
	}
	defer errorFile.Close()
	errorWritten := false

	start := time.Now()
	results := make([]StepResult, 0, len(steps))
	runnerLogs := make([]string, 0, len(steps)*2+2)

	writeLine := func(line string) {
		tagged := fmt.Sprintf("%s %s", elapsedTag(start, time.Now()), line)
		fmt.Println(tagged)
		_, _ = fmt.Fprintln(logFile, tagged)
		runnerLogs = append(runnerLogs, tagged)
	}
	writeError := func(line string) {
		errorWritten = true
		_, _ = fmt.Fprintln(errorFile, line)
	}

	for _, s := range steps {
		writeLine(fmt.Sprintf("[TEST] START %s", s.Name))
		startStepLogCapture()

		stepTimeout := s.Timeout
		if stepTimeout == 0 {
			stepTimeout = 30 * time.Second
		}

		ctx, cancel := context.WithTimeout(context.Background(), stepTimeout)

		stepStart := time.Now()
		var output string
		var err error
		var stepReport string

		// Run the step in a goroutine to support context cancellation
		done := make(chan struct{})
		go func() {
			runner := s.Run
			if s.RunWithContext != nil {
				runner = func() error {
					out, runErr := s.RunWithContext(&StepContext{Name: s.Name, Started: stepStart})
					if runErr == nil {
						stepReport = strings.TrimSpace(out.Report)
					}
					return runErr
				}
			}
			output, err = captureStepOutput(start, runner)
			close(done)
		}()

		select {
		case <-done:
			cancel()
		case <-ctx.Done():
			cancel()
			err = fmt.Errorf("step timed out after %v", stepTimeout)
		}

		duration := time.Since(stepStart)
		aboutLine := fmt.Sprintf("%s [TEST] RUN   %s", elapsedTag(start, stepStart), s.Name)
		trimmedOutput := strings.TrimSpace(output)
		if trimmedOutput == "" {
			trimmedOutput = aboutLine
		} else {
			trimmedOutput = aboutLine + "\n" + trimmedOutput
		}

		res := StepResult{
			Name:        s.Name,
			Passed:      err == nil,
			Duration:    duration,
			SectionID:   s.SectionID,
			Screenshot:  s.Screenshot,
			Screenshots: normalizedStepScreenshots(s),
			Report:      stepReport,
		}
		if len(res.Screenshots) == 1 && res.Screenshot == "" {
			res.Screenshot = res.Screenshots[0]
		}
		if len(res.Screenshots) > 1 || strings.TrimSpace(s.ScreenshotGrid) != "" {
			gridPath, gridErr := buildScreenshotGrid(res.Screenshots, s.ScreenshotGrid, options.ReportPath)
			if gridErr != nil && err == nil {
				err = fmt.Errorf("build screenshot grid: %w", gridErr)
				res.Passed = false
			}
			if gridErr == nil && gridPath != "" {
				res.ScreenshotGrid = gridPath
				res.Screenshot = gridPath
			}
		}
		if err != nil {
			res.Error = err.Error()
		}
		res.Logs = endStepLogCapture()
		res.Output = trimmedOutput
		if trimmedStepOutput := strings.TrimSpace(output); trimmedStepOutput != "" {
			for _, rawLine := range strings.Split(trimmedStepOutput, "\n") {
				line := strings.TrimSpace(rawLine)
				if line == "" {
					continue
				}
				_, _ = fmt.Fprintln(logFile, line)
				runnerLogs = append(runnerLogs, line)
			}
		}
		results = append(results, res)
		for _, entry := range res.Logs {
			if entry.Level == "error" || entry.Level == "exception" {
				writeError(fmt.Sprintf("%s [%s] (%s) %s", elapsedTag(start, entry.At), entry.Level, s.Name, entry.Message))
			}
		}

		if err != nil {
			writeLine(fmt.Sprintf("[TEST] FAIL  %s: %v", s.Name, err))
			writeError(fmt.Sprintf("%s [TEST] FAIL  %s: %v", elapsedTag(start, time.Now()), s.Name, err))
			_ = writeReport(options, results, time.Since(start), runnerLogs, start)
			if !errorWritten {
				_, _ = fmt.Fprintln(errorFile, "(no errors)")
			}
			return err
		}
		writeLine(fmt.Sprintf("[TEST] PASS  %s", s.Name))
	}

	writeLine("[TEST] COMPLETE")
	if !errorWritten {
		_, _ = fmt.Fprintln(errorFile, "(no errors)")
	}
	return writeReport(options, results, time.Since(start), runnerLogs, start)
}

func writeReport(options SuiteOptions, results []StepResult, total time.Duration, runnerLogs []string, suiteStart time.Time) error {
	status := "✅ PASS"
	for _, r := range results {
		if !r.Passed {
			status = "❌ FAIL"
			break
		}
	}

	reportDir := filepath.Dir(options.ReportPath)
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return err
	}

	f, err := os.Create(options.ReportPath)
	if err != nil {
		return err
	}
	defer f.Close()

	title := options.Version + " Test Report"
	if strings.Contains(options.ReportPath, "plugins/") {
		parts := strings.Split(options.ReportPath, "/")
		for i, p := range parts {
			if p == "plugins" && i+1 < len(parts) {
				title = strings.Title(parts[i+1]) + " Plugin " + options.Version + " Test Report"
				break
			}
		}
	}

	_, _ = fmt.Fprintf(f, "# %s\n", title)
	_, _ = fmt.Fprintln(f)
	_, _ = fmt.Fprintf(f, "**Generated at:** %s\n", time.Now().Format(time.RFC1123Z))
	_, _ = fmt.Fprintf(f, "**Version:** `%s`\n", options.Version)
	_, _ = fmt.Fprintln(f, "**Runner:** `test_v2`")
	_, _ = fmt.Fprintf(f, "**Status:** %s\n", status)
	_, _ = fmt.Fprintf(f, "**Total Time:** `%s`\n", total.Round(time.Millisecond))
	_, _ = fmt.Fprintln(f)
	_, _ = fmt.Fprintln(f, "## Test Steps")
	_, _ = fmt.Fprintln(f)
	_, _ = fmt.Fprintln(f, "| Step | Result | Duration |")
	_, _ = fmt.Fprintln(f, "|---|---|---|")
	for _, r := range results {
		stepStatus := "✅ PASS"
		if !r.Passed {
			stepStatus = "❌ FAIL"
		}
		_, _ = fmt.Fprintf(f, "| %s | %s | `%s` |\n", r.Name, stepStatus, r.Duration.Round(time.Millisecond))
	}

	_, _ = fmt.Fprintln(f)
	_, _ = fmt.Fprintln(f, "## Step Logs")
	_, _ = fmt.Fprintln(f)
	for _, r := range results {
		_, _ = fmt.Fprintf(f, "### %s\n\n", r.Name)
		_, _ = fmt.Fprintln(f, "```text")
		_, _ = fmt.Fprintf(f, "result: %s\n", map[bool]string{true: "PASS", false: "FAIL"}[r.Passed])
		_, _ = fmt.Fprintf(f, "duration: %s\n", r.Duration.Round(time.Millisecond))
		if r.SectionID != "" {
			_, _ = fmt.Fprintf(f, "section: %s\n", r.SectionID)
		}
		if r.Error != "" {
			_, _ = fmt.Fprintf(f, "error: %s\n", r.Error)
		}
		_, _ = fmt.Fprintln(f, "```")
		_, _ = fmt.Fprintln(f)
		if strings.TrimSpace(r.Report) != "" {
			_, _ = fmt.Fprintln(f, "#### Step Story")
			_, _ = fmt.Fprintln(f)
			_, _ = fmt.Fprintln(f, r.Report)
			_, _ = fmt.Fprintln(f)
		}
		_, _ = fmt.Fprintln(f, "#### Runner Output")
		_, _ = fmt.Fprintln(f)
		_, _ = fmt.Fprintln(f, "```text")
		if r.Output == "" {
			_, _ = fmt.Fprintln(f, "(no output)")
		} else {
			_, _ = fmt.Fprintln(f, r.Output)
		}
		_, _ = fmt.Fprintln(f, "```")
		_, _ = fmt.Fprintln(f)
		filteredLogs := filterLogsForStep(r.Logs, r.SectionID, options.BrowserLogMode)
		if len(filteredLogs) > 0 {
			_, _ = fmt.Fprintln(f, "#### Browser Logs")
			_, _ = fmt.Fprintln(f)
			_, _ = fmt.Fprintln(f, "```text")
			for _, entry := range filteredLogs {
				_, _ = fmt.Fprintf(f, "%s [%s] %s\n", elapsedTag(suiteStart, entry.At), entry.Level, entry.Message)
			}
			_, _ = fmt.Fprintln(f, "```")
			_, _ = fmt.Fprintln(f)

			hasBrowserErrors := false
			for _, entry := range filteredLogs {
				if entry.Level == "error" || entry.Level == "exception" {
					hasBrowserErrors = true
					break
				}
			}
			if hasBrowserErrors {
				_, _ = fmt.Fprintln(f, "#### Browser Errors")
				_, _ = fmt.Fprintln(f)
				_, _ = fmt.Fprintln(f, "```text")
				for _, entry := range filteredLogs {
					if entry.Level == "error" || entry.Level == "exception" {
						_, _ = fmt.Fprintf(f, "%s [%s] %s\n", elapsedTag(suiteStart, entry.At), entry.Level, entry.Message)
					}
				}
				_, _ = fmt.Fprintln(f, "```")
				_, _ = fmt.Fprintln(f)
			}
		}
		if r.ScreenshotGrid != "" {
			_, _ = fmt.Fprintf(f, "![%s sequence](../%s)\n\n", r.Name, r.ScreenshotGrid)
		} else if r.Screenshot != "" {
			_, _ = fmt.Fprintf(f, "![%s](../%s)\n\n", r.Name, r.Screenshot)
		}
	}

	_, _ = fmt.Fprintln(f, "## Artifacts")
	_, _ = fmt.Fprintln(f)
	_, _ = fmt.Fprintln(f, "- `test.log`")
	_, _ = fmt.Fprintln(f, "- `error.log`")
	wrote := map[string]bool{}
	for _, r := range results {
		if r.ScreenshotGrid != "" && !wrote[r.ScreenshotGrid] {
			_, _ = fmt.Fprintf(f, "- `%s`\n", r.ScreenshotGrid)
			wrote[r.ScreenshotGrid] = true
		}
		for _, shot := range r.Screenshots {
			if shot != "" && !wrote[shot] {
				_, _ = fmt.Fprintf(f, "- `%s`\n", shot)
				wrote[shot] = true
			}
		}
		if r.Screenshot != "" && !wrote[r.Screenshot] {
			_, _ = fmt.Fprintf(f, "- `%s`\n", r.Screenshot)
			wrote[r.Screenshot] = true
		}
	}

	return nil
}

func normalizedStepScreenshots(s Step) []string {
	if len(s.Screenshots) > 0 {
		out := make([]string, 0, len(s.Screenshots))
		seen := map[string]bool{}
		for _, shot := range s.Screenshots {
			trimmed := strings.TrimSpace(shot)
			if trimmed == "" || seen[trimmed] {
				continue
			}
			seen[trimmed] = true
			out = append(out, trimmed)
		}
		return out
	}
	if strings.TrimSpace(s.Screenshot) != "" {
		return []string{strings.TrimSpace(s.Screenshot)}
	}
	return nil
}

func buildScreenshotGrid(refs []string, outRef string, reportPath string) (string, error) {
	validRefs := make([]string, 0, len(refs))
	validPaths := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		absPath := resolveScreenshotRefPath(reportPath, ref)
		if _, err := os.Stat(absPath); err == nil {
			validRefs = append(validRefs, ref)
			validPaths = append(validPaths, absPath)
		}
	}
	if len(validPaths) == 0 {
		return "", fmt.Errorf("no screenshot files found")
	}
	if len(validPaths) == 1 && strings.TrimSpace(outRef) == "" {
		return validRefs[0], nil
	}

	if strings.TrimSpace(outRef) == "" {
		first := validRefs[0]
		ext := filepath.Ext(first)
		base := strings.TrimSuffix(first, ext)
		outRef = base + "_grid.png"
	}
	outPath := resolveScreenshotRefPath(reportPath, outRef)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return "", err
	}

	firstImage, err := decodeImage(validPaths[0])
	if err != nil {
		return "", err
	}
	tileW := firstImage.Bounds().Dx()
	tileH := firstImage.Bounds().Dy()
	if tileW <= 0 || tileH <= 0 {
		return "", fmt.Errorf("invalid screenshot bounds")
	}

	cols := len(validPaths)
	rows := 1
	canvas := image.NewRGBA(image.Rect(0, 0, tileW*cols, tileH*rows))

	for i, shotPath := range validPaths {
		img, err := decodeImage(shotPath)
		if err != nil {
			return "", err
		}
		x := (i % cols) * tileW
		y := (i / cols) * tileH
		draw.Draw(canvas, image.Rect(x, y, x+tileW, y+tileH), img, img.Bounds().Min, draw.Src)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := png.Encode(f, canvas); err != nil {
		return "", err
	}

	return outRef, nil
}

func decodeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func resolveScreenshotRefPath(reportPath string, ref string) string {
	if filepath.IsAbs(ref) {
		return ref
	}
	reportDir := filepath.Dir(reportPath)
	primary := filepath.Join(reportDir, ref)
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	parent := filepath.Join(filepath.Dir(reportDir), ref)
	if _, err := os.Stat(parent); err == nil {
		return parent
	}
	if strings.HasPrefix(ref, "screenshots"+string(filepath.Separator)) || strings.HasPrefix(ref, "screenshots/") {
		parentScreens := filepath.Join(filepath.Dir(reportDir), "screenshots")
		if stat, err := os.Stat(parentScreens); err == nil && stat.IsDir() {
			return filepath.Join(filepath.Dir(reportDir), ref)
		}
	}
	return primary
}
