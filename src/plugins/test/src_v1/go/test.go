package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dialtone/dev/plugins/chrome/src_v1/go"
	"dialtone/dev/plugins/logs/src_v1/go"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/nats-io/nats.go"
)

type Step struct {
	Name           string
	RunWithContext func(*StepContext) (StepRunResult, error)
	SectionID      string
	Screenshots    []string
	ScreenshotGrid string
	Timeout        time.Duration
}

type StepContext struct {
	Name         string
	Started      time.Time
	Session      *BrowserSession
	LogWriter    io.Writer
	SuiteSubject string
	StepSubject  string
	ErrorSubject string
	natsURL      string
	logger       *logs.NATSLogger
	errorLogger  *logs.NATSLogger
}

func (sc *StepContext) Logf(format string, args ...any) {
	sc.Infof(format, args...)
}

func (sc *StepContext) Infof(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logs.Info("[STEP:%s] %s", sc.Name, msg)
	if sc.logger != nil {
		_ = sc.logger.Infof("%s", msg)
	}
}

func (sc *StepContext) Warnf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logs.Warn("[STEP:%s] %s", sc.Name, msg)
	if sc.logger != nil {
		_ = sc.logger.Warnf("%s", msg)
	}
}

func (sc *StepContext) Debugf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logs.Debug("[STEP:%s] %s", sc.Name, msg)
	if sc.logger != nil {
		_ = sc.logger.Infof("%s", msg)
	}
}

func (sc *StepContext) Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logs.Error("[STEP:%s] %s", sc.Name, msg)
	if sc.logger != nil {
		_ = sc.logger.Errorf("%s", msg)
	}
	if sc.errorLogger != nil {
		_ = sc.errorLogger.Errorf("[STEP:%s] %s", sc.Name, msg)
	}
}

func (sc *StepContext) WaitForMessage(subject string, pattern string, timeout time.Duration) error {
	if sc.logger == nil || sc.logger.Conn() == nil {
		return fmt.Errorf("NATS not available in this test context")
	}
	nc := sc.logger.Conn()

	msgCh := make(chan string, 100)
	sub, err := nc.Subscribe(subject, func(m *nats.Msg) {
		msgCh <- logs.FormatMessage(m.Subject, m.Data)
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	deadline := time.Now().Add(timeout)
	for {
		select {
		case data := <-msgCh:
			if strings.Contains(data, pattern) {
				return nil
			}
		case <-time.After(time.Until(deadline)):
			return fmt.Errorf("timeout waiting for %q on %s", pattern, subject)
		}
	}
}

func (sc *StepContext) NATSConn() *nats.Conn {
	if sc.logger == nil {
		return nil
	}
	return sc.logger.Conn()
}

func (sc *StepContext) NATSURL() string {
	return strings.TrimSpace(sc.natsURL)
}

func (sc *StepContext) NewTopicLogger(subject string) (*logs.NATSLogger, error) {
	nc := sc.NATSConn()
	if nc == nil {
		return nil, fmt.Errorf("NATS not available in this test context")
	}
	return logs.NewNATSLogger(nc, subject)
}

func (sc *StepContext) WaitForMessageAfterAction(subject, pattern string, timeout time.Duration, action func() error) error {
	if sc.logger == nil || sc.logger.Conn() == nil {
		return fmt.Errorf("NATS not available in this test context")
	}
	nc := sc.logger.Conn()
	msgCh := make(chan string, 100)
	sub, err := nc.Subscribe(subject, func(m *nats.Msg) {
		msgCh <- logs.FormatMessage(m.Subject, m.Data)
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	if err := nc.Flush(); err != nil {
		return err
	}
	if err := action(); err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	for {
		select {
		case data := <-msgCh:
			if strings.Contains(data, pattern) {
				return nil
			}
		case <-time.After(time.Until(deadline)):
			return fmt.Errorf("timeout waiting for %q on %s", pattern, subject)
		}
	}
}

func (sc *StepContext) WaitForAllMessagesAfterAction(subject string, patterns []string, timeout time.Duration, action func() error) error {
	if sc.logger == nil || sc.logger.Conn() == nil {
		return fmt.Errorf("NATS not available in this test context")
	}
	if len(patterns) == 0 {
		return fmt.Errorf("no patterns provided")
	}
	nc := sc.logger.Conn()
	msgCh := make(chan string, 100)
	sub, err := nc.Subscribe(subject, func(m *nats.Msg) {
		msgCh <- logs.FormatMessage(m.Subject, m.Data)
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	if err := nc.Flush(); err != nil {
		return err
	}
	if err := action(); err != nil {
		return err
	}
	seen := map[string]bool{}
	deadline := time.Now().Add(timeout)
	for len(seen) < len(patterns) {
		select {
		case data := <-msgCh:
			for _, p := range patterns {
				if !seen[p] && strings.Contains(data, p) {
					seen[p] = true
				}
			}
		case <-time.After(time.Until(deadline)):
			missing := []string{}
			for _, p := range patterns {
				if !seen[p] {
					missing = append(missing, p)
				}
			}
			return fmt.Errorf("timeout waiting for patterns on %s: %s", subject, strings.Join(missing, ", "))
		}
	}
	return nil
}

func (sc *StepContext) WaitForStepMessage(pattern string, timeout time.Duration) error {
	if strings.TrimSpace(sc.StepSubject) == "" {
		return fmt.Errorf("step subject not available in this test context")
	}
	return sc.WaitForMessage(sc.StepSubject, pattern, timeout)
}

func (sc *StepContext) WaitForErrorMessage(pattern string, timeout time.Duration) error {
	if strings.TrimSpace(sc.ErrorSubject) == "" {
		return fmt.Errorf("error subject not available in this test context")
	}
	return sc.WaitForMessage(sc.ErrorSubject, pattern, timeout)
}

func (sc *StepContext) WaitForErrorMessageAfterAction(pattern string, timeout time.Duration, action func() error) error {
	if strings.TrimSpace(sc.ErrorSubject) == "" {
		return fmt.Errorf("error subject not available in this test context")
	}
	return sc.WaitForMessageAfterAction(sc.ErrorSubject, pattern, timeout, action)
}

func (sc *StepContext) WaitForStepMessageAfterAction(pattern string, timeout time.Duration, action func() error) error {
	if strings.TrimSpace(sc.StepSubject) == "" {
		return fmt.Errorf("step subject not available in this test context")
	}
	return sc.WaitForMessageAfterAction(sc.StepSubject, pattern, timeout, action)
}

func (sc *StepContext) ResetStepLogClock() {
	if strings.TrimSpace(sc.StepSubject) == "" {
		return
	}
	logs.ResetTopicClock(sc.StepSubject)
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
	NATSURL        string
	NATSSubject    string
	AutoStartNATS  bool
}

type ConsoleMessage struct {
	Type string
	Text string
	Time time.Time
}

type BrowserSession struct {
	ctx      context.Context
	cancel   context.CancelFunc
	Session  *chrome.Session
	mu       sync.Mutex
	messages []ConsoleMessage
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

func (s *BrowserSession) HasConsoleMessage(substr string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.messages {
		if strings.Contains(m.Text, substr) {
			return true
		}
	}
	return false
}

func (s *BrowserSession) Entries() []ConsoleMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]ConsoleMessage(nil), s.messages...)
}

func ConnectToBrowser(port int, role string) (*BrowserSession, error) {
	wsURL, err := getWebsocketURL(port)
	if err != nil {
		return nil, fmt.Errorf("failed to get websocket URL for port %d: %w", port, err)
	}

	session := &chrome.Session{
		PID:          0, // Unknown
		Port:         port,
		WebSocketURL: wsURL,
		IsNew:        false,
	}

	return initSession(session, role)
}

func getWebsocketURL(port int) (string, error) {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var data struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	return data.WebSocketDebuggerURL, nil
}

func initSession(session *chrome.Session, role string) (*BrowserSession, error) {
	logs.Info("   [BROWSER] Connecting to WebSocket: %s", session.WebSocketURL)
	// Connect to the browser via websocket
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(context.Background(), session.WebSocketURL)

	// Create context
	ctx, cancelCtx := chromedp.NewContext(allocCtx)

	s := &BrowserSession{
		ctx:     ctx,
		cancel:  func() { cancelCtx(); cancelAlloc() },
		Session: session,
	}

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			var text strings.Builder
			for i, arg := range ev.Args {
				if i > 0 {
					text.WriteString(" ")
				}
				text.WriteString(string(arg.Value))
			}
			msg := ConsoleMessage{
				Type: string(ev.Type),
				Text: text.String(),
				Time: time.Now(),
			}
			s.mu.Lock()
			s.messages = append(s.messages, msg)
			s.mu.Unlock()
			logs.Info("   [BROWSER CONSOLE | PID %d] %s: %s", session.PID, ev.Type, msg.Text)
		case *runtime.EventExceptionThrown:
			logs.Error("   [BROWSER EXCEPTION | PID %d] %s", session.PID, ev.ExceptionDetails.Text)
		}
	})

	return s, nil
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

	s, err := initSession(session, opts.Role)
	if err != nil {
		return nil, err
	}

	if opts.URL != "" {
		logs.Info("   [BROWSER] Navigating to: %s", opts.URL)
		if err := chromedp.Run(s.ctx, chromedp.Navigate(opts.URL)); err != nil {
			s.Close()
			return nil, err
		}
	}

	return s, nil
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
			logs.SetOutput(f)
			defer f.Close()
		}
	}
	if opts.ErrorLogPath != "" {
		// Truncate error log
		_ = os.WriteFile(opts.ErrorLogPath, []byte(""), 0644)
	}

	logs.Info("Starting Test Suite: %s", opts.Version)

	natsURL := strings.TrimSpace(opts.NATSURL)
	if natsURL == "" {
		natsURL = "nats://127.0.0.1:4222"
	}
	autoStart := true
	if opts.AutoStartNATS {
		autoStart = true
	}
	nc, broker, baseSubject, natsErr := setupSuiteNATS(opts, natsURL, autoStart)
	if natsErr != nil {
		logs.Warn("NATS suite logging disabled: %v", natsErr)
	}
	if nc != nil {
		defer nc.Close()
	}
	if broker != nil {
		defer broker.Close()
	}

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
			Name:         step.Name,
			Started:      time.Now(),
			SuiteSubject: baseSubject,
			ErrorSubject: baseSubject + ".error",
			natsURL:      natsURL,
		}
		if nc != nil {
			stepSubject := baseSubject + "." + sanitizeSubjectToken(step.Name)
			if stepLogger, err := logs.NewNATSLogger(nc, stepSubject); err == nil {
				stepCtx.logger = stepLogger
				stepCtx.StepSubject = stepSubject
				if errLogger, eerr := logs.NewNATSLogger(nc, stepCtx.ErrorSubject); eerr == nil {
					stepCtx.errorLogger = errLogger
				}
				stepCtx.Logf("step started")
			}
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
			stepCtx.Errorf("step timed out after %v", timeout)
			// Try to capture a timeout screenshot if we have a session
			// We need a way to access the session here.
			// For now, let's just log.
		case <-done:
			if err != nil {
				logs.Error("Step %s failed: %v", step.Name, err)
				stepCtx.Errorf("step failed: %v", err)
			} else if result.Report != "" {
				logs.Info("Step %s report: %s", step.Name, result.Report)
				stepCtx.Logf("report: %s", result.Report)
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

func setupSuiteNATS(opts SuiteOptions, natsURL string, autoStart bool) (*nats.Conn, *logs.EmbeddedNATS, string, error) {
	tryConnect := func(url string) (*nats.Conn, error) {
		return nats.Connect(url, nats.Timeout(1200*time.Millisecond))
	}
	nc, err := tryConnect(natsURL)
	if err != nil && autoStart {
		broker, berr := logs.StartEmbeddedNATSOnURL(natsURL)
		if berr != nil {
			return nil, nil, "", berr
		}
		nc = broker.Conn()
		if nc == nil {
			broker.Close()
			return nil, nil, "", fmt.Errorf("embedded nats connection not available")
		}
		subj := strings.TrimSpace(opts.NATSSubject)
		if subj == "" {
			subj = "logs.test." + sanitizeSubjectToken(opts.Version)
		}
		logs.Info("Suite NATS logging active at %s subject=%s (embedded=true)", broker.URL(), subj)
		return nc, broker, subj, nil
	}
	if err != nil {
		return nil, nil, "", err
	}
	subj := strings.TrimSpace(opts.NATSSubject)
	if subj == "" {
		subj = "logs.test." + sanitizeSubjectToken(opts.Version)
	}
	logs.Info("Suite NATS logging active at %s subject=%s", natsURL, subj)
	return nc, nil, subj, nil
}

func sanitizeSubjectToken(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	repl := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", "|", "-", ":", "-", ".", "-", "_", "-", "(", "", ")", "", "'", "", "\"", "")
	s = repl.Replace(s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" {
		return "default"
	}
	return s
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
