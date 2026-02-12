package test_v2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
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

func StartBrowser(opts BrowserOptions) (*BrowserSession, error) {
	s := &BrowserSession{}
	if opts.Role == "" {
		opts.Role = "default"
	}

	resolved, err := chrome_app.StartSession(chrome_app.SessionOptions{
		GPU:           true,
		Headless:      opts.Headless,
		TargetURL:     "",
		Role:          opts.Role,
		ReuseExisting: opts.ReuseExisting,
	})
	if err != nil {
		return nil, err
	}

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), resolved.WebSocketURL)
	ctx, ctxCancel := chromedp.NewContext(allocCtx)
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
			entry = &ConsoleEntry{Level: string(e.Type), Message: formatConsoleArgs(e.Args)}
		case *runtime.EventExceptionThrown:
			msg := e.ExceptionDetails.Text
			if e.ExceptionDetails.Exception != nil && e.ExceptionDetails.Exception.Description != "" {
				msg = e.ExceptionDetails.Exception.Description
			}
			entry = &ConsoleEntry{Level: "exception", Message: msg}
		}
		if entry == nil {
			return
		}
		s.mu.Lock()
		s.entries = append(s.entries, *entry)
		s.mu.Unlock()
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

type Step struct {
	Name       string
	Run        func() error
	SectionID  string
	Screenshot string
}

type SuiteOptions struct {
	Version    string
	ReportPath string
	LogPath    string
}

type StepResult struct {
	Name       string
	Passed     bool
	Error      string
	Duration   time.Duration
	SectionID  string
	Screenshot string
}

func RunSuite(options SuiteOptions, steps []Step) error {
	logFile, err := os.Create(options.LogPath)
	if err != nil {
		return err
	}
	defer logFile.Close()

	start := time.Now()
	results := make([]StepResult, 0, len(steps))
	runnerLogs := make([]string, 0, len(steps)*2+2)

	writeLine := func(line string) {
		fmt.Println(line)
		_, _ = fmt.Fprintln(logFile, line)
		runnerLogs = append(runnerLogs, line)
	}

	for _, s := range steps {
		writeLine(fmt.Sprintf("[TEST] START %s", s.Name))

		stepStart := time.Now()
		err := s.Run()
		duration := time.Since(stepStart)

		res := StepResult{
			Name:       s.Name,
			Passed:     err == nil,
			Duration:   duration,
			SectionID:  s.SectionID,
			Screenshot: s.Screenshot,
		}
		if err != nil {
			res.Error = err.Error()
		}
		results = append(results, res)

		if err != nil {
			writeLine(fmt.Sprintf("[TEST] FAIL  %s: %v", s.Name, err))
			_ = writeReport(options, results, time.Since(start), runnerLogs)
			return err
		}
		writeLine(fmt.Sprintf("[TEST] PASS  %s", s.Name))
	}

	writeLine("[TEST] COMPLETE")
	return writeReport(options, results, time.Since(start), runnerLogs)
}

func writeReport(options SuiteOptions, results []StepResult, total time.Duration, runnerLogs []string) error {
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

	_, _ = fmt.Fprintln(f, "# Template Plugin v3 Test Report")
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
		if r.Screenshot != "" {
			_, _ = fmt.Fprintf(f, "![%s](../%s)\n\n", r.Name, r.Screenshot)
		}
	}

	_, _ = fmt.Fprintln(f, "## Artifacts")
	_, _ = fmt.Fprintln(f)
	_, _ = fmt.Fprintln(f, "- `test.log`")
	_, _ = fmt.Fprintln(f, "- `error.log`")
	for _, r := range results {
		if r.Screenshot != "" {
			_, _ = fmt.Fprintf(f, "- `%s`\n", r.Screenshot)
		}
	}
	_, _ = fmt.Fprintln(f)
	_, _ = fmt.Fprintln(f, "## Raw Runner Log")
	_, _ = fmt.Fprintln(f)
	_, _ = fmt.Fprintln(f, "```text")
	for _, line := range runnerLogs {
		_, _ = fmt.Fprintln(f, line)
	}
	_, _ = fmt.Fprintln(f, "```")

	return nil
}
