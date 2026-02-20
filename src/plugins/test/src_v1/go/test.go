package test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dialtone/dev/plugins/chrome/app"
	"dialtone/dev/plugins/logs/src_v1/go"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type Step struct {
	Name            string
	RunWithContext  func(*StepContext) (StepRunResult, error)
	SectionID       string
	Screenshots     []string
	ScreenshotGrid  string
	Timeout         time.Duration
}

type StepContext struct {
	Name      string
	Started   time.Time
	Session   *BrowserSession
	LogWriter io.Writer
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

type BrowserSession struct {
	ctx     context.Context
	cancel  context.CancelFunc
	Session *chrome.Session
}

func (s *BrowserSession) Context() context.Context {
	return s.ctx
}

func (s *BrowserSession) ChromeSession() *chrome.Session {
	return s.Session
}

func (s *BrowserSession) Close() {
	s.cancel()
	if s.Session != nil && s.Session.IsNew {
		chrome.CleanupSession(s.Session)
	}
}

func (s *BrowserSession) Run(tasks ...chromedp.Action) error {
	return chromedp.Run(s.ctx, tasks...)
}

func (s *BrowserSession) CaptureScreenshot(path string) error {
	var buf []byte
	if err := chromedp.Run(s.ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0644)
}

type BrowserOptions struct {
	Headless      bool
	GPU           bool
	Role          string
	ReuseExisting bool
	URL           string
	LogWriter     io.Writer
	LogPrefix     string
}

func StartBrowser(opts BrowserOptions) (*BrowserSession, error) {
	logs.Info("   [BROWSER] Starting session (role=%s, reuse=%v, gpu=%v)...", opts.Role, opts.ReuseExisting, opts.GPU)
	session, err := chrome.StartSession(chrome.SessionOptions{
		Headless:      opts.Headless,
		GPU:           opts.GPU,
		Role:          opts.Role,
		ReuseExisting: opts.ReuseExisting,
		TargetURL:     opts.URL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start chrome session: %w", err)
	}

	logs.Info("   [BROWSER] Connecting to WebSocket: %s", session.WebSocketURL)
	// Connect to the browser via websocket
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(context.Background(), session.WebSocketURL)
	
	// Create context
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			for _, arg := range ev.Args {
				logs.Info("   [BROWSER CONSOLE | PID %d] %s: %s", session.PID, ev.Type, arg.Value)
			}
		case *runtime.EventExceptionThrown:
			logs.Error("   [BROWSER EXCEPTION | PID %d] %s", session.PID, ev.ExceptionDetails.Text)
		}
	})
	
	if opts.URL != "" {
		logs.Info("   [BROWSER] Navigating to: %s", opts.URL)
		if err := chromedp.Run(ctx, chromedp.Navigate(opts.URL)); err != nil {
			cancelCtx()
			cancelAlloc()
			return nil, err
		}
	}

	return &BrowserSession{
		ctx:     ctx,
		cancel:  func() { cancelCtx(); cancelAlloc() },
		Session: session,
	}, nil
}

type StepResult struct {
	Step   Step
	Error  error
	Result StepRunResult
	Start  time.Time
	End    time.Time
}

func RunSuite(opts SuiteOptions, steps []Step) error {
	if opts.LogPath != "" {
		// Truncate and open log file
		f, err := os.OpenFile(opts.LogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err == nil {
			mw := io.MultiWriter(os.Stdout, f)
			logs.SetOutput(mw)
			defer f.Close()
		}
	}
	if opts.ErrorLogPath != "" {
		// Truncate error log
		_ = os.WriteFile(opts.ErrorLogPath, []byte(""), 0644)
	}

	logs.Info("Starting Test Suite: %s", opts.Version)
	
	startTime := time.Now()
	var results []StepResult
	
	for i, step := range steps {
		logs.Info("[%d/%d] Running step: %s", i+1, len(steps), step.Name)
		
		timeout := step.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		
		stepCtx := &StepContext{
			Name:    step.Name,
			Started: time.Now(),
		}
		
		done := make(chan struct{})
		var result StepRunResult
		var err error
		
		go func() {
			result, err = step.RunWithContext(stepCtx)
			close(done)
		}()
		
		select {
		case <-ctx.Done():
			logs.Error("Step %s timed out after %v", step.Name, timeout)
			err = fmt.Errorf("step %s timed out", step.Name)
			// Try to capture a timeout screenshot if we have a session
			// We need a way to access the session here. 
			// For now, let's just log.
		case <-done:
			if err != nil {
				logs.Error("Step %s failed: %v", step.Name, err)
			} else if result.Report != "" {
				logs.Info("Step %s report: %s", step.Name, result.Report)
			}
		}
		
		results = append(results, StepResult{
			Step:   step,
			Error:  err,
			Result: result,
			Start:  stepCtx.Started,
			End:    time.Now(),
		})
		
		if err != nil {
			break
		}
	}
	
	duration := time.Since(startTime)
	logs.Info("Test Suite Completed in %v", duration)
	
	if opts.ReportPath != "" {
		if genErr := generateReport(opts, results, duration); genErr != nil {
			logs.Error("Failed to generate report: %v", genErr)
		}
	}
	
	for _, r := range results {
		if r.Error != nil {
			return r.Error
		}
	}
	return nil
}

func generateReport(opts SuiteOptions, results []StepResult, totalDuration time.Duration) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Test Report: %s\n\n", opts.Version))
	sb.WriteString(fmt.Sprintf("- **Date**: %s\n", time.Now().Format(time.RFC1123)))
	sb.WriteString(fmt.Sprintf("- **Total Duration**: %v\n\n", totalDuration))
	
	sb.WriteString("## Summary\n\n")
	passed := 0
	for _, r := range results {
		if r.Error == nil {
			passed++
		}
	}
	sb.WriteString(fmt.Sprintf("- **Steps**: %d / %d passed\n", passed, len(results)))
	status := "PASSED"
	if passed < len(results) {
		status = "FAILED"
	}
	sb.WriteString(fmt.Sprintf("- **Status**: %s\n\n", status))
	
	sb.WriteString("## Details\n\n")
	for i, r := range results {
		icon := "✅"
		if r.Error != nil {
			icon = "❌"
		}
		sb.WriteString(fmt.Sprintf("### %d. %s %s\n\n", i+1, icon, r.Step.Name))
		sb.WriteString(fmt.Sprintf("- **Duration**: %v\n", r.End.Sub(r.Start)))
		if r.Error != nil {
			sb.WriteString(fmt.Sprintf("- **Error**: `%v`\n", r.Error))
		}
		if r.Result.Report != "" {
			sb.WriteString(fmt.Sprintf("- **Report**: %s\n", r.Result.Report))
		}
		
		if len(r.Step.Screenshots) > 0 {
			sb.WriteString("\n#### Screenshots\n\n")
			for _, s := range r.Step.Screenshots {
				// Use relative path for markdown
				fname := filepath.Base(s)
				sb.WriteString(fmt.Sprintf("![%s](screenshots/%s)\n", fname, fname))
			}
		}
		sb.WriteString("\n---\n\n")
	}
	
	return os.WriteFile(opts.ReportPath, []byte(sb.String()), 0644)
}

// Re-export common chromedp things to avoid direct dependency if possible
type Action = chromedp.Action

func Sleep(d time.Duration) Action {
	return chromedp.Sleep(d)
}

func Navigate(url string) Action {
	return chromedp.Navigate(url)
}
