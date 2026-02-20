package test

import (
	"context"
	"fmt"
	"io"
	"os"
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
				logs.Info("   [BROWSER CONSOLE] %s: %s", ev.Type, arg.Value)
			}
		case *runtime.EventExceptionThrown:
			logs.Error("   [BROWSER EXCEPTION] %s", ev.ExceptionDetails.Text)
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

func RunSuite(opts SuiteOptions, steps []Step) error {
	logs.Info("Starting Test Suite: %s", opts.Version)
	
	startTime := time.Now()
	
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
			return fmt.Errorf("step %s timed out", step.Name)
		case <-done:
			if err != nil {
				logs.Error("Step %s failed: %v", step.Name, err)
				return err
			}
			if result.Report != "" {
				logs.Info("Step %s report: %s", step.Name, result.Report)
			}
		}
	}
	
	logs.Info("Test Suite Completed in %v", time.Since(startTime))
	return nil
}

// Re-export common chromedp things to avoid direct dependency if possible
type Action = chromedp.Action

func Sleep(d time.Duration) Action {
	return chromedp.Sleep(d)
}

func Navigate(url string) Action {
	return chromedp.Navigate(url)
}
