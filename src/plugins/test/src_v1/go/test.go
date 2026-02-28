package test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	stdruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"dialtone/dev/plugins/chrome/src_v1/go"
	"dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	cdruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/nats-io/nats.go"
)

const testTag = "[TEST]"
const defaultStepTimeout = 10 * time.Second

var srcVersionTestDirRe = regexp.MustCompile(`(?i)(.*[/\\]src_v[0-9]+)[/\\]test(?:[/\\].*)?$`)
var callerSrcVersionRootRe = regexp.MustCompile(`(?i)(.*?/src/plugins/[^/]+/src_v[0-9]+)(?:/.*)?$`)

type Step struct {
	Name           string
	RunWithContext func(*StepContext) (StepRunResult, error)
	SectionID      string
	Screenshots    []string
	ScreenshotGrid string
	Timeout        time.Duration
}

type StepContext struct {
	Name            string
	Started         time.Time
	Session         *BrowserSession
	LogWriter       io.Writer
	SuiteSubject    string
	StepSubject     string
	BrowserSubject  string
	ErrorSubject    string
	natsURL         string
	logger          *logs.NATSLogger
	browserLogger   *logs.NATSLogger
	errorLogger     *logs.NATSLogger
	passLogger      *logs.NATSLogger
	failLogger      *logs.NATSLogger
	suiteBrowser    *BrowserSession
	setSuiteBrowser func(*BrowserSession)
	repoRoot        string
	stepLogs        []string
	stepErrors      []string
	browserLogs     []string
	stepScreenshots []string
	browserUsed     bool
	reportPath      string
	autoShotDone    bool
	logMu           sync.Mutex
}

func (sc *StepContext) Logf(format string, args ...any) {
	sc.Infof(format, args...)
}

func (sc *StepContext) Infof(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	sc.appendStepLog("INFO", msg)
	source := sc.callerLocation()
	logs.InfoFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	if sc.logger != nil {
		_ = sc.logger.InfofFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	}
}

func (sc *StepContext) Warnf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	sc.appendStepLog("WARN", msg)
	source := sc.callerLocation()
	logs.WarnFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	if sc.logger != nil {
		_ = sc.logger.WarnfFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	}
}

func (sc *StepContext) Debugf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	sc.appendStepLog("DEBUG", msg)
	source := sc.callerLocation()
	logs.DebugFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	if sc.logger != nil {
		_ = sc.logger.InfofFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	}
}

func (sc *StepContext) Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	sc.appendStepError("ERROR", msg)
	source := sc.callerLocation()
	logs.ErrorFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	if sc.logger != nil {
		_ = sc.logger.ErrorfFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	}
	if sc.errorLogger != nil {
		_ = sc.errorLogger.ErrorfFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	}
}

func (sc *StepContext) TestPassf(format string, args ...any) {
	msg := strings.TrimSpace(fmt.Sprintf(format, args...))
	if msg == "" {
		msg = "step passed"
	}
	source := sc.callerLocation()
	line := fmt.Sprintf("[TEST][PASS] [STEP:%s] %s", sc.Name, msg)
	sc.appendStepLog("PASS", line)
	logs.InfoFromTest(source, "%s", line)
	if sc.passLogger != nil {
		_ = sc.passLogger.InfofFromTest(source, "%s", line)
		return
	}
	if sc.logger != nil {
		_ = sc.logger.InfofFromTest(source, "%s", line)
	}
}

func (sc *StepContext) TestFailf(format string, args ...any) {
	msg := strings.TrimSpace(fmt.Sprintf(format, args...))
	if msg == "" {
		msg = "step failed"
	}
	source := sc.callerLocation()
	line := fmt.Sprintf("[TEST][FAIL] [STEP:%s] %s", sc.Name, msg)
	sc.appendStepError("FAIL", line)
	logs.ErrorFromTest(source, "%s", line)
	if sc.failLogger != nil {
		_ = sc.failLogger.ErrorfFromTest(source, "%s", line)
		return
	}
	if sc.errorLogger != nil {
		_ = sc.errorLogger.ErrorfFromTest(source, "%s", line)
		return
	}
	if sc.logger != nil {
		_ = sc.logger.ErrorfFromTest(source, "%s", line)
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

func (sc *StepContext) NATSURLForHost(host string) (string, error) {
	base := strings.TrimSpace(sc.natsURL)
	host = strings.TrimSpace(host)
	if base == "" {
		return "", fmt.Errorf("NATS not configured in this test context")
	}
	if host == "" {
		return "", fmt.Errorf("host is required")
	}
	trimmed := strings.TrimSpace(strings.TrimPrefix(base, "nats://"))
	parts := strings.Split(trimmed, ":")
	port := "4222"
	if len(parts) > 1 {
		port = strings.TrimSpace(parts[len(parts)-1])
	}
	if port == "" {
		port = "4222"
	}
	return fmt.Sprintf("nats://%s:%s", host, port), nil
}

func (sc *StepContext) RepoRoot() string {
	return strings.TrimSpace(sc.repoRoot)
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

func (sc *StepContext) WaitForBrowserMessage(pattern string, timeout time.Duration) error {
	if strings.TrimSpace(sc.BrowserSubject) == "" {
		return fmt.Errorf("browser subject not available in this test context")
	}
	return sc.WaitForMessage(sc.BrowserSubject, pattern, timeout)
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

func (sc *StepContext) WaitForBrowserMessageAfterAction(pattern string, timeout time.Duration, action func() error) error {
	if strings.TrimSpace(sc.BrowserSubject) == "" {
		return fmt.Errorf("browser subject not available in this test context")
	}
	return sc.WaitForMessageAfterAction(sc.BrowserSubject, pattern, timeout, action)
}

func (sc *StepContext) ResetStepLogClock() {
	if strings.TrimSpace(sc.StepSubject) == "" {
		return
	}
	logs.ResetTopicClock(sc.StepSubject)
}

func (sc *StepContext) callerLocation() string {
	for i := 2; i < 14; i++ {
		_, file, line, ok := stdruntime.Caller(i)
		if !ok {
			break
		}
		norm := filepath.ToSlash(file)
		if strings.Contains(norm, "/plugins/test/src_v1/go/test.go") {
			continue
		}
		if idx := strings.Index(norm, "/src/"); idx >= 0 {
			if line > 0 {
				return fmt.Sprintf("%s:%d", norm[idx+1:], line)
			}
			return norm[idx+1:]
		}
		base := filepath.Base(file)
		if line > 0 {
			return fmt.Sprintf("%s:%d", base, line)
		}
		return base
	}
	return "unknown"
}

func (sc *StepContext) appendStepLog(level, msg string) {
	line := strings.TrimSpace(fmt.Sprintf("%s: %s", strings.TrimSpace(level), strings.TrimSpace(msg)))
	if line == "" {
		return
	}
	sc.logMu.Lock()
	sc.stepLogs = append(sc.stepLogs, line)
	sc.logMu.Unlock()
}

func (sc *StepContext) appendStepError(level, msg string) {
	line := strings.TrimSpace(fmt.Sprintf("%s: %s", strings.TrimSpace(level), strings.TrimSpace(msg)))
	if line == "" {
		return
	}
	sc.logMu.Lock()
	sc.stepErrors = append(sc.stepErrors, line)
	sc.logMu.Unlock()
}

func (sc *StepContext) snapshotStepLogs() ([]string, []string, []string) {
	sc.logMu.Lock()
	defer sc.logMu.Unlock()
	logCopy := append([]string(nil), sc.stepLogs...)
	errCopy := append([]string(nil), sc.stepErrors...)
	browserCopy := append([]string(nil), sc.browserLogs...)
	return logCopy, errCopy, browserCopy
}

func (sc *StepContext) appendBrowserLog(kind, msg string, isError bool) {
	line := strings.TrimSpace(fmt.Sprintf("%s: %s", strings.TrimSpace(kind), strings.TrimSpace(msg)))
	if line == "" {
		return
	}
	if isError {
		line = "ERROR: " + line
	} else {
		line = "INFO: " + line
	}
	sc.logMu.Lock()
	sc.browserLogs = append(sc.browserLogs, line)
	sc.logMu.Unlock()
}

type StepRunResult struct {
	Report string
}

type SuiteOptions struct {
	Version               string
	RepoRoot              string
	ReportPath            string
	RawReportPath         string
	ReportFormat          string
	ReportTitle           string
	ReportRunner          string
	ChromeReportNode      string
	LogPath               string
	ErrorLogPath          string
	BrowserLogMode        string
	PreserveSharedBrowser bool
	SkipBrowserCleanup    bool
	BrowserCleanupRole    string
	NATSURL               string
	NATSListenURL         string
	NATSSubject           string
	AutoStartNATS         bool
}

type ConsoleMessage struct {
	Type string
	Text string
	Time time.Time
}

type BrowserSession struct {
	ctx          context.Context
	cancel       context.CancelFunc
	Session      *chrome.Session
	closers      []io.Closer
	mu           sync.Mutex
	messages     []ConsoleMessage
	onConsole    func(ConsoleMessage)
	onError      func(ConsoleMessage)
	mainTargetID target.ID
	allowCreate  bool
}

func (s *BrowserSession) Context() context.Context {
	return s.ctx
}

func (s *BrowserSession) ChromeSession() *chrome.Session {
	return s.Session
}

func (s *BrowserSession) Close() {
	s.cancel()
	for _, c := range s.closers {
		if c != nil {
			_ = c.Close()
		}
	}
	if s.Session != nil && s.Session.IsNew {
		chrome.CleanupSession(s.Session)
	}
}

func (s *BrowserSession) Run(tasks ...chromedp.Action) error {
	if err := chromedp.Run(s.ctx, tasks...); err != nil {
		if isRecoverableBrowserRunError(err) {
			logs.Warn("   [BROWSER] recoverable run error; attempting rebind: %v", err)
			if rerr := s.EnsureOpenPage(); rerr == nil {
				if err2 := chromedp.Run(s.ctx, tasks...); err2 == nil {
					return nil
				} else if isNoTargetIDError(err2) {
					logs.Warn("   [BROWSER] rebind retry hit stale target; ensuring page and retrying once more")
					if epErr := s.EnsureOpenPage(); epErr == nil {
						return chromedp.Run(s.ctx, tasks...)
					} else {
						logs.Warn("   [BROWSER] ensure open page after stale target failed: %v", epErr)
					}
				} else {
					logs.Warn("   [BROWSER] rebind retry failed: %v", err2)
					return err2
				}
			} else {
				logs.Warn("   [BROWSER] recoverable run error but rebind failed: %v", rerr)
			}
		}
		return err
	}
	return nil
}

func (s *BrowserSession) RunWithTimeout(timeout time.Duration, tasks ...chromedp.Action) error {
	if timeout <= 0 {
		return s.Run(tasks...)
	}
	ctx, cancel := context.WithTimeout(s.ctx, timeout)
	defer cancel()
	if err := chromedp.Run(ctx, tasks...); err != nil {
		if isRecoverableBrowserRunError(err) {
			logs.Warn("   [BROWSER] recoverable run-with-timeout error; attempting rebind: %v", err)
			if rerr := s.EnsureOpenPage(); rerr == nil {
				ctx2, cancel2 := context.WithTimeout(s.ctx, timeout)
				defer cancel2()
				if err2 := chromedp.Run(ctx2, tasks...); err2 == nil {
					return nil
				} else if isNoTargetIDError(err2) {
					logs.Warn("   [BROWSER] rebind retry (timeout) hit stale target; ensuring page and retrying once more")
					if epErr := s.EnsureOpenPage(); epErr == nil {
						ctx3, cancel3 := context.WithTimeout(s.ctx, timeout)
						defer cancel3()
						return chromedp.Run(ctx3, tasks...)
					} else {
						logs.Warn("   [BROWSER] ensure open page after stale target (timeout) failed: %v", epErr)
					}
				} else {
					logs.Warn("   [BROWSER] rebind retry (timeout) failed: %v", err2)
					return err2
				}
			} else {
				logs.Warn("   [BROWSER] recoverable run-with-timeout error but rebind failed: %v", rerr)
			}
		}
		return err
	}
	return nil
}

func isNoBrowserOpenError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(text, "no browser is open") || strings.Contains(text, "failed to open new tab")
}

func isRecoverableBrowserRunError(err error) bool {
	if err == nil {
		return false
	}
	if isNoBrowserOpenError(err) {
		return true
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(text, "target closed") ||
		strings.Contains(text, "context canceled") ||
		strings.Contains(text, "no target with given id found") ||
		strings.Contains(text, "(-32602)")
}

func isNoTargetIDError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(text, "no target with given id found") || strings.Contains(text, "(-32602)")
}

func (s *BrowserSession) EnsureOpenPage() error {
	if s == nil || s.Session == nil {
		return fmt.Errorf("browser session unavailable")
	}
	s.mu.Lock()
	allowCreate := s.allowCreate
	s.mu.Unlock()
	if targetID, err := s.ensureFirstPageTargetIDViaCDP(allowCreate); err == nil && strings.TrimSpace(targetID) != "" {
		return s.rebindToTarget(targetID)
	}
	ports := candidateDebugPortsForSession(s.Session)
	if len(ports) == 0 {
		return fmt.Errorf("browser session debug port unavailable")
	}
	var lastErr error
	for _, p := range ports {
		host := "127.0.0.1"
		if s.Session != nil {
			host = debugHostFromWebSocketURL(s.Session.WebSocketURL)
		}
		targetID, err := ensureFirstPageTargetIDAt(host, p, allowCreate)
		if err != nil {
			lastErr = err
			continue
		}
		return s.rebindToTarget(targetID)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no usable debug port")
	}
	return lastErr
}

func (s *BrowserSession) ensureFirstPageTargetIDViaCDP(allowCreate bool) (string, error) {
	if s == nil || s.ctx == nil {
		return "", fmt.Errorf("browser session unavailable")
	}
	chromeCtx := chromedp.FromContext(s.ctx)
	if chromeCtx == nil || chromeCtx.Browser == nil {
		return "", fmt.Errorf("browser executor unavailable")
	}
	browserExecCtx := cdp.WithExecutor(s.ctx, chromeCtx.Browser)
	targets, err := target.GetTargets().Do(browserExecCtx)
	if err != nil {
		return "", err
	}
	for _, t := range targets {
		if t == nil || t.Type != "page" {
			continue
		}
		if strings.TrimSpace(string(t.TargetID)) == "" {
			continue
		}
		return string(t.TargetID), nil
	}
	if !allowCreate {
		return "", fmt.Errorf("no page target found")
	}
	targetURL := strings.TrimSpace(RuntimeConfigSnapshot().BrowserNewTargetURL)
	if targetURL == "" {
		targetURL = "about:blank"
	}
	created, err := target.CreateTarget(targetURL).Do(browserExecCtx)
	if err == nil && strings.TrimSpace(string(created)) != "" {
		return string(created), nil
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		targets, terr := target.GetTargets().Do(browserExecCtx)
		if terr == nil {
			for _, t := range targets {
				if t == nil || t.Type != "page" {
					continue
				}
				if strings.TrimSpace(string(t.TargetID)) == "" {
					continue
				}
				return string(t.TargetID), nil
			}
		}
		time.Sleep(120 * time.Millisecond)
	}
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("no page target found")
}

func candidateDebugPortsForSession(session *chrome.Session) []int {
	if session == nil {
		return nil
	}
	seen := map[int]struct{}{}
	out := make([]int, 0, 2)
	add := func(p int) {
		if p <= 0 {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	if p := debugPortFromWebSocketURL(session.WebSocketURL); p > 0 {
		add(p)
	}
	add(session.Port)
	return out
}

func debugPortFromWebSocketURL(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	u, err := url.Parse(raw)
	if err != nil {
		return 0
	}
	host := strings.TrimSpace(u.Host)
	if host == "" {
		return 0
	}
	_, portStr, err := net.SplitHostPort(host)
	if err != nil {
		// Handle host:port without brackets for some malformed values.
		if idx := strings.LastIndex(host, ":"); idx > -1 && idx+1 < len(host) {
			portStr = host[idx+1:]
		} else {
			return 0
		}
	}
	port, err := strconv.Atoi(strings.TrimSpace(portStr))
	if err != nil || port <= 0 {
		return 0
	}
	return port
}

func (s *BrowserSession) rebindToTarget(targetID string) error {
	targetID = strings.TrimSpace(targetID)
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(context.Background(), s.Session.WebSocketURL)
	ctxOpts := []chromedp.ContextOption{}
	if targetID != "" {
		ctxOpts = append(ctxOpts, chromedp.WithTargetID(target.ID(targetID)))
	}
	ctx, cancelCtx := chromedp.NewContext(allocCtx, ctxOpts...)
	oldCancel := s.cancel
	s.ctx = ctx
	s.cancel = func() { cancelCtx(); cancelAlloc() }
	s.mainTargetID = target.ID(targetID)
	s.attachTargetListener(ctx)
	if oldCancel != nil {
		oldCancel()
	}
	return nil
}

func (s *BrowserSession) ensureMainTargetID() error {
	s.mu.Lock()
	if s.mainTargetID != "" {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()
	if err := chromedp.Run(s.ctx); err != nil {
		return err
	}
	chromeCtx := chromedp.FromContext(s.ctx)
	if chromeCtx == nil || chromeCtx.Target == nil {
		return fmt.Errorf("unable to resolve current browser target")
	}
	s.mu.Lock()
	s.mainTargetID = chromeCtx.Target.TargetID
	s.mu.Unlock()
	return nil
}

func (s *BrowserSession) CloseExtraTabsKeepMain() error {
	if err := s.ensureMainTargetID(); err != nil {
		return err
	}
	chromeCtx := chromedp.FromContext(s.ctx)
	if chromeCtx == nil || chromeCtx.Browser == nil {
		return fmt.Errorf("browser executor unavailable for tab cleanup")
	}
	browserExecCtx := cdp.WithExecutor(s.ctx, chromeCtx.Browser)
	s.mu.Lock()
	mainID := s.mainTargetID
	s.mu.Unlock()
	targets, err := target.GetTargets().Do(browserExecCtx)
	if err != nil {
		return err
	}
	for _, t := range targets {
		if t == nil || t.Type != "page" {
			continue
		}
		if t.TargetID == mainID {
			continue
		}
		_ = target.CloseTarget(t.TargetID).Do(browserExecCtx)
	}
	return nil
}

func (s *BrowserSession) CaptureScreenshot(path string) error {
	var buf []byte
	if err := chromedp.Run(s.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		data, err := page.CaptureScreenshot().
			WithCaptureBeyondViewport(false).
			WithFromSurface(false).
			Do(ctx)
		if err != nil {
			return err
		}
		buf = data
		return nil
	})); err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0644)
}

type BrowserOptions struct {
	Headless            bool
	GPU                 bool
	Role                string
	ReuseExisting       bool
	UserDataDir         string
	URL                 string
	SkipNavigateOnReuse bool
	PreserveTabAndSize  bool
	RemoteNode          string
	LogWriter           io.Writer
	LogPrefix           string
}

func RemoteBrowserConfigured() bool {
	return strings.TrimSpace(RuntimeConfigSnapshot().BrowserNode) != ""
}

func BrowserProviderAvailable() bool {
	return chrome.FindChromePath() != "" || RemoteBrowserConfigured()
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
	cfg := RuntimeConfigSnapshot()
	remoteMode := strings.TrimSpace(cfg.BrowserNode) != ""
	if session != nil {
		if session.Port <= 0 {
			if p := debugPortFromWebSocketURL(session.WebSocketURL); p > 0 {
				session.Port = p
			}
		}
		if !remoteMode && strings.TrimSpace(session.WebSocketURL) != "" {
			if fixed := forceLocalWebSocketHost(session.WebSocketURL); strings.TrimSpace(fixed) != "" {
				session.WebSocketURL = fixed
			}
		}
	}
	if session != nil && session.Port > 0 && (!remoteMode || isLocalWebSocketHost(session.WebSocketURL)) {
		if ws, err := getWebsocketURL(session.Port); err == nil && strings.TrimSpace(ws) != "" {
			session.WebSocketURL = strings.TrimSpace(ws)
		}
	}
	logs.Info("   [BROWSER] Connecting to WebSocket: %s", session.WebSocketURL)
	// Connect to the browser via websocket
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(context.Background(), session.WebSocketURL)

	// Reuse an existing page target when possible to avoid opening additional tabs.
	// This also applies to remote/tunneled sessions because the debug port is exposed locally.
	ctxOpts := []chromedp.ContextOption{}
	allowCreate := session.IsNew || cfg.BrowserAllowCreateTarget || strings.TrimSpace(cfg.BrowserNode) != ""
	if session.Port > 0 {
		debugHost := debugHostFromWebSocketURL(session.WebSocketURL)
		if targetID, err := ensureFirstPageTargetIDAt(debugHost, session.Port, allowCreate); err == nil && targetID != "" {
			ctxOpts = append(ctxOpts, chromedp.WithTargetID(target.ID(targetID)))
		}
	}
	ctx, cancelCtx := chromedp.NewContext(allocCtx, ctxOpts...)

	s := &BrowserSession{
		ctx:         ctx,
		cancel:      func() { cancelCtx(); cancelAlloc() },
		Session:     session,
		allowCreate: allowCreate,
	}
	s.attachTargetListener(ctx)
	return s, nil
}

func isLocalWebSocketHost(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil {
		return false
	}
	host := strings.TrimSpace(strings.ToLower(u.Hostname()))
	return host == "127.0.0.1" || host == "localhost"
}

func debugHostFromWebSocketURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil {
		return "127.0.0.1"
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return "127.0.0.1"
	}
	return host
}

func forceLocalWebSocketHost(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil {
		return raw
	}
	if !strings.EqualFold(u.Scheme, "ws") && !strings.EqualFold(u.Scheme, "wss") {
		return raw
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" || host == "127.0.0.1" || strings.EqualFold(host, "localhost") {
		return raw
	}
	// In WSL NAT mode, DevTools may only be reachable on the Windows gateway IP.
	// Keep that host intact instead of forcing localhost.
	if stdruntime.GOOS == "linux" {
		if gw := wslGatewayIP(); gw != "" && host == gw {
			return raw
		}
	}
	port := strings.TrimSpace(u.Port())
	if port == "" {
		return raw
	}
	u.Host = net.JoinHostPort("127.0.0.1", port)
	return u.String()
}

func wslGatewayIP() string {
	out, err := exec.Command("sh", "-lc", "ip route | awk '/^default / {print $3; exit}'").Output()
	if err != nil {
		return ""
	}
	ip := strings.TrimSpace(string(out))
	if ip == "" || ip == "100.100.100.100" {
		return ""
	}
	return ip
}

func (s *BrowserSession) attachTargetListener(ctx context.Context) {
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *cdruntime.EventConsoleAPICalled:
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
			if s.onConsole != nil {
				s.onConsole(msg)
			}
			pid := 0
			if s.Session != nil {
				pid = s.Session.PID
			}
			logs.Info("   [BROWSER CONSOLE | PID %d] %s: %s", pid, ev.Type, msg.Text)
		case *cdruntime.EventExceptionThrown:
			msgText := strings.TrimSpace(ev.ExceptionDetails.Text)
			if ev.ExceptionDetails.Exception != nil {
				desc := strings.TrimSpace(ev.ExceptionDetails.Exception.Description)
				if desc != "" {
					if msgText == "" {
						msgText = desc
					} else if !strings.Contains(msgText, desc) {
						msgText = msgText + " " + desc
					}
				}
			}
			if msgText == "" {
				msgText = "javascript exception"
			}
			exMsg := ConsoleMessage{
				Type: "exception",
				Text: msgText,
				Time: time.Now(),
			}
			s.mu.Lock()
			s.messages = append(s.messages, exMsg)
			s.mu.Unlock()
			if s.onError != nil {
				s.onError(exMsg)
			}
			pid := 0
			if s.Session != nil {
				pid = s.Session.PID
			}
			logs.Error("   [BROWSER EXCEPTION | PID %d] %s", pid, ev.ExceptionDetails.Text)
		}
	})
}

func getFirstPageTargetID(port int) (string, error) {
	return getFirstPageTargetIDAt("127.0.0.1", port)
}

func getFirstPageTargetIDAt(host string, port int) (string, error) {
	if port <= 0 {
		return "", fmt.Errorf("invalid port")
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	resp, err := http.Get(fmt.Sprintf("http://%s/json/list", net.JoinHostPort(host, strconv.Itoa(port))))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	type pageTarget struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	var targets []pageTarget
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return "", err
	}
	for _, t := range targets {
		if t.Type == "page" && strings.TrimSpace(t.ID) != "" {
			return t.ID, nil
		}
	}
	return "", fmt.Errorf("no page target found")
}

func createTargetAtPort(port int, url string) error {
	return createTargetAtPortAt("127.0.0.1", port, url)
}

func createTargetAtPortAt(host string, port int, url string) error {
	if port <= 0 {
		return fmt.Errorf("invalid port")
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	u := fmt.Sprintf("http://%s/json/new?%s", net.JoinHostPort(host, strconv.Itoa(port)), url)
	req, err := http.NewRequest(http.MethodPut, u, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		_ = resp.Body.Close()
		if resp.StatusCode < 400 {
			return nil
		}
	}
	// Fallback for older endpoints that allow GET.
	resp, err = http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("create target status %d", resp.StatusCode)
	}
	return nil
}

func ensureFirstPageTargetID(port int, allowCreate bool) (string, error) {
	return ensureFirstPageTargetIDAt("127.0.0.1", port, allowCreate)
}

func ensureFirstPageTargetIDAt(host string, port int, allowCreate bool) (string, error) {
	if targetID, err := getFirstPageTargetIDAt(host, port); err == nil && strings.TrimSpace(targetID) != "" {
		return targetID, nil
	}
	if !allowCreate {
		return "", fmt.Errorf("no page target found")
	}
	targetURL := strings.TrimSpace(RuntimeConfigSnapshot().BrowserNewTargetURL)
	if targetURL == "" {
		targetURL = "about:blank"
	}
	if err := createTargetAtPortAt(host, port, targetURL); err != nil {
		return "", err
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if targetID, err := getFirstPageTargetIDAt(host, port); err == nil && strings.TrimSpace(targetID) != "" {
			return targetID, nil
		}
		time.Sleep(120 * time.Millisecond)
	}
	return "", fmt.Errorf("no page target found after creating about:blank")
}

func StartBrowser(opts BrowserOptions) (*BrowserSession, error) {
	navigateOnStart := strings.TrimSpace(opts.URL) != "" && !opts.SkipNavigateOnReuse && !opts.PreserveTabAndSize
	if remoteNode := resolveRemoteBrowserNode(opts); remoteNode != "" {
		logs.Info("   [BROWSER] Remote node configured; trying remote-first on %s", remoteNode)
		rs, rerr := startRemoteBrowser(remoteNode, opts)
		if rerr == nil {
			if navigateOnStart {
				if err := chromedp.Run(rs.ctx, chromedp.Navigate(opts.URL)); err != nil {
					if opts.ReuseExisting {
						errText := strings.ToLower(err.Error())
						if strings.Contains(errText, "no browser is open") || strings.Contains(errText, "failed to open new tab") {
							if epErr := rs.EnsureOpenPage(); epErr == nil {
								if nerr := chromedp.Run(rs.ctx, chromedp.Navigate(opts.URL)); nerr == nil {
									return rs, nil
								}
							}
						}
						// Keep the currently attached headed browser; do not spawn/attach again.
						logs.Warn("   [BROWSER] remote reused session navigate failed on %s; continuing with existing attached session", remoteNode)
						return rs, nil
					}
					rs.Close()
					return nil, err
				}
			}
			return rs, nil
		}
		if RuntimeConfigSnapshot().NoSSH {
			logs.Warn("   [BROWSER] remote-first failed on %s in no-ssh mode; not falling back to local", remoteNode)
			return nil, fmt.Errorf("remote-first failed on %s in no-ssh mode: %w", remoteNode, rerr)
		}
		logs.Warn("   [BROWSER] remote-first failed on %s; falling back to local start", remoteNode)
	}

	cfg := RuntimeConfigSnapshot()
	requestedPort := cfg.RemoteDebugPort
	debugAddress := ""
	wslMode := stdruntime.GOOS == "linux" && wslGatewayIP() != ""
	if requestedPort > 0 {
		if wslMode {
			// With Windows portproxy on fixed ports, Chrome must bind loopback.
			debugAddress = "127.0.0.1"
		}
		isChromeDebug, isInUse := probeRequestedDebugPort(requestedPort)
		if isChromeDebug {
			logs.Info("   [BROWSER] requested debug port %d already serves Chrome DevTools; reusing it", requestedPort)
			s, err := ConnectToBrowser(requestedPort, opts.Role)
			if err == nil {
				if navigateOnStart {
					if err := chromedp.Run(s.ctx, chromedp.Navigate(opts.URL)); err != nil {
						s.Close()
						return nil, err
					}
				}
				return s, nil
			}
			logs.Warn("   [BROWSER] reuse existing debug port %d failed; falling back to launch path: %v", requestedPort, err)
		} else if isInUse && !opts.ReuseExisting {
			if wslMode {
				logs.Warn("   [BROWSER] requested debug port %d is in use (likely WSL proxy listener); keeping it and launching Chrome on loopback", requestedPort)
			} else {
				logs.Warn("   [BROWSER] requested debug port %d in use by non-Chrome endpoint; cleaning it before launch", requestedPort)
				if err := chrome.CleanupPort(requestedPort); err != nil {
					return nil, fmt.Errorf("cleanup occupied requested debug port %d: %w", requestedPort, err)
				}
			}
		}
	}

	logs.Info("   [BROWSER] Starting session (role=%s, reuse=%v, gpu=%v)...", opts.Role, opts.ReuseExisting, opts.GPU)
	session, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: requestedPort,
		Headless:      opts.Headless,
		GPU:           opts.GPU,
		Role:          opts.Role,
		ReuseExisting: opts.ReuseExisting,
		UserDataDir:   opts.UserDataDir,
		TargetURL:     opts.URL,
		DebugAddress:  debugAddress,
	})
	if err != nil {
		// First fallback: attach to an already-running Dialtone browser for this role/headless mode.
		if attach := findAttachableDialtoneSession(opts.Role, opts.Headless); attach != nil {
			s, aerr := initSession(attach, opts.Role)
			if aerr == nil {
				if navigateOnStart {
					if navErr := chromedp.Run(s.ctx, chromedp.Navigate(opts.URL)); navErr != nil {
						s.Close()
						return nil, navErr
					}
				}
				return s, nil
			}
		}
		// Second fallback: force a fresh launch (disable reuse) after a short settle delay.
		time.Sleep(300 * time.Millisecond)
		session, err = chrome.StartSession(chrome.SessionOptions{
			RequestedPort: requestedPort,
			Headless:      opts.Headless,
			GPU:           opts.GPU,
			Role:          opts.Role,
			ReuseExisting: false,
			UserDataDir:   opts.UserDataDir,
			TargetURL:     opts.URL,
			DebugAddress:  debugAddress,
		})
		if err != nil {
			if remoteNode := resolveRemoteBrowserNode(opts); remoteNode != "" {
				logs.Warn("   [BROWSER] local launch failed; trying remote node %s", remoteNode)
				rs, rerr := startRemoteBrowser(remoteNode, opts)
				if rerr == nil {
					if navigateOnStart {
						if err := chromedp.Run(rs.ctx, chromedp.Navigate(opts.URL)); err != nil {
							rs.Close()
							return nil, err
						}
					}
					return rs, nil
				}
				return nil, fmt.Errorf("failed local start (%v), remote fallback failed on %s (%v)", err, remoteNode, rerr)
			}
			return nil, fmt.Errorf("failed to start chrome session: %w", err)
		}
	}

	s, err := initSession(session, opts.Role)
	if err != nil {
		if remoteNode := resolveRemoteBrowserNode(opts); remoteNode != "" {
			logs.Warn("   [BROWSER] local session attach failed; trying remote node %s", remoteNode)
			rs, rerr := startRemoteBrowser(remoteNode, opts)
			if rerr == nil {
				if navigateOnStart {
					if err := chromedp.Run(rs.ctx, chromedp.Navigate(opts.URL)); err != nil {
						rs.Close()
						return nil, err
					}
				}
				return rs, nil
			}
			return nil, fmt.Errorf("local session init failed (%v), remote fallback failed on %s (%v)", err, remoteNode, rerr)
		}
		return nil, err
	}

	if navigateOnStart {
		logs.Info("   [BROWSER] Navigating to: %s", opts.URL)
		if err := chromedp.Run(s.ctx, chromedp.Navigate(opts.URL)); err != nil {
			s.Close()
			if remoteNode := resolveRemoteBrowserNode(opts); remoteNode != "" {
				logs.Warn("   [BROWSER] local navigate failed; trying remote node %s", remoteNode)
				rs, rerr := startRemoteBrowser(remoteNode, opts)
				if rerr == nil {
					if navigateOnStart {
						if nerr := chromedp.Run(rs.ctx, chromedp.Navigate(opts.URL)); nerr != nil {
							rs.Close()
							return nil, nerr
						}
					}
					return rs, nil
				}
				return nil, fmt.Errorf("local navigate failed (%v), remote fallback failed on %s (%v)", err, remoteNode, rerr)
			}
			return nil, err
		}
	}

	return s, nil
}

func probeRequestedDebugPort(port int) (isChromeDebug bool, isInUse bool) {
	if port <= 0 {
		return false, false
	}
	hosts := []string{"127.0.0.1"}
	if stdruntime.GOOS == "linux" {
		if gw := wslGatewayIP(); gw != "" {
			hosts = append([]string{gw}, hosts...)
		}
	}
	isReachable := false
	for _, host := range hosts {
		if canDialHostPort(host, port, 600*time.Millisecond) {
			isReachable = true
			if isChromeDevToolsEndpoint(host, port) {
				return true, true
			}
		}
	}
	return false, isReachable
}

func isChromeDevToolsEndpoint(host string, port int) bool {
	client := &http.Client{Timeout: 900 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://%s:%d/json/version", strings.TrimSpace(host), port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var data struct {
		Browser              string `json:"Browser"`
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return false
	}
	b := strings.ToLower(strings.TrimSpace(data.Browser))
	if b == "" || strings.TrimSpace(data.WebSocketDebuggerURL) == "" {
		return false
	}
	return strings.Contains(b, "chrome") || strings.Contains(b, "chromium") || strings.Contains(b, "edge")
}

func resolveRemoteBrowserNode(opts BrowserOptions) string {
	if n := strings.TrimSpace(opts.RemoteNode); n != "" {
		return n
	}
	return strings.TrimSpace(RuntimeConfigSnapshot().BrowserNode)
}

func startRemoteBrowser(node string, opts BrowserOptions) (*BrowserSession, error) {
	nodeInfo, err := sshv1.ResolveMeshNode(node)
	if err != nil {
		return nil, err
	}
	transport, _ := sshv1.ResolveCommandTransport(nodeInfo.Name)
	// For Windows nodes controlled via local PowerShell transport from WSL,
	// skip tailnet-first probing and use the Windows-aware launch/attach path.
	if strings.EqualFold(nodeInfo.OS, "windows") && strings.EqualFold(strings.TrimSpace(transport), "powershell") {
		return startRemoteBrowserWindows(nodeInfo, opts)
	}
	if s, derr := startRemoteBrowserDirectTailnet(nodeInfo, opts); derr == nil {
		logs.Info("   [BROWSER] tailnet direct attach succeeded on %s", nodeInfo.Name)
		return s, nil
	} else {
		logs.Warn("   [BROWSER] tailnet direct attach unavailable on %s: %v", nodeInfo.Name, derr)
	}
	if RuntimeConfigSnapshot().NoSSH {
		return startRemoteBrowserDirectTailnet(nodeInfo, opts)
	}
	if strings.EqualFold(nodeInfo.OS, "windows") {
		return startRemoteBrowserWindows(nodeInfo, opts)
	}
	// Prefer a no-repo SSH flow: reuse an already-debuggable browser on the remote host,
	// or launch one directly there if needed. This avoids requiring Dialtone source on node.
	if s, err := startRemoteBrowserUnixNoRepo(nodeInfo, opts); err == nil {
		return s, nil
	} else {
		logs.Warn("   [BROWSER] remote no-repo attach path failed on %s: %v", nodeInfo.Name, err)
	}
	role := strings.TrimSpace(opts.Role)
	if role == "" {
		role = "test"
	}
	url := strings.TrimSpace(opts.URL)
	if url == "" {
		url = "about:blank"
	}
	candidates := make([]string, 0, len(nodeInfo.RepoCandidates)+5)
	seen := map[string]struct{}{}
	addCandidate := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		candidates = append(candidates, v)
	}
	addCandidate("$HOME/dialtone")
	for _, c := range nodeInfo.RepoCandidates {
		addCandidate(c)
	}
	addCandidate("/home/user/dialtone")
	addCandidate("/home/tim/dialtone")
	addCandidate("/mnt/c/Users/tim/dialtone")
	addCandidate("/mnt/c/Users/timca/dialtone")
	candidateExpr := strings.Join(candidates, " ")
	cmd := fmt.Sprintf("repo=''; for d in %s; do if [ -d \"$d\" ]; then repo=\"$d\"; break; fi; done; if [ -z \"$repo\" ]; then echo 'dialtone repo not found on remote node'; exit 1; fi; cd \"$repo\" && ./dialtone.sh chrome src_v1 session --role %s --headless=%t --gpu=%t --reuse-existing=%t --debug-address 0.0.0.0 --url %s",
		candidateExpr, shellQuote(role), opts.Headless, opts.GPU, opts.ReuseExisting, shellQuote(url))
	out, err := sshv1.RunNodeCommand(nodeInfo.Name, cmd, sshv1.CommandOptions{})
	if err != nil {
		return nil, fmt.Errorf("remote command on %s failed: %v output=%s", nodeInfo.Name, err, strings.TrimSpace(out))
	}
	raw := extractChromeSessionJSON(out)
	if raw == "" {
		return nil, fmt.Errorf("remote chrome session output missing metadata marker")
	}
	var meta chrome.SessionMetadata
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil, fmt.Errorf("decode remote chrome session metadata: %w", err)
	}
	if meta.DebugPort <= 0 {
		return nil, fmt.Errorf("invalid remote debug port %d", meta.DebugPort)
	}
	wsPath := strings.TrimSpace(meta.WebSocketPath)
	if wsPath == "" {
		wsPath = chrome.WebSocketPathFromURL(meta.WebSocketURL)
	}
	if wsPath == "" {
		return nil, fmt.Errorf("remote websocket path is empty")
	}
	attachHost := strings.TrimSpace(nodeInfo.Host)
	attachPort := meta.DebugPort
	var tunnelCloser io.Closer
	if attachHost == "" || !canDialHostPort(attachHost, attachPort, 1500*time.Millisecond) {
		if closer, lport, err := openSSHDebugTunnel(nodeInfo, meta.DebugPort); err == nil {
			attachHost = "127.0.0.1"
			attachPort = lport
			tunnelCloser = closer
			if localWS, werr := getWebsocketURL(attachPort); werr == nil && strings.TrimSpace(localWS) != "" {
				if p := chrome.WebSocketPathFromURL(localWS); strings.TrimSpace(p) != "" {
					wsPath = p
				}
			}
		}
	}
	if strings.TrimSpace(attachHost) == "" {
		return nil, fmt.Errorf("remote attach host is empty")
	}
	session := &chrome.Session{
		PID:          meta.PID,
		Port:         attachPort,
		WebSocketURL: fmt.Sprintf("ws://%s:%d%s", attachHost, attachPort, wsPath),
		IsNew:        false,
	}
	s, err := initSession(session, role)
	if err != nil {
		if tunnelCloser != nil {
			_ = tunnelCloser.Close()
		}
		return nil, err
	}
	if tunnelCloser != nil {
		s.closers = append(s.closers, tunnelCloser)
	}
	return s, nil
}

func startRemoteBrowserDirectTailnet(nodeInfo sshv1.MeshNode, opts BrowserOptions) (*BrowserSession, error) {
	host := strings.TrimSpace(nodeInfo.Host)
	if host == "" {
		return nil, fmt.Errorf("tailnet host unavailable for node %s", nodeInfo.Name)
	}
	ports := make([]int, 0, 4)
	addPort := func(p int) {
		if p <= 0 {
			return
		}
		for _, e := range ports {
			if e == p {
				return
			}
		}
		ports = append(ports, p)
	}
	cfg := RuntimeConfigSnapshot()
	if cfg.RemoteDebugPort > 0 {
		addPort(cfg.RemoteDebugPort)
	}
	for _, p := range cfg.RemoteDebugPorts {
		if p > 0 {
			addPort(p)
		}
	}
	addPort(chrome.DefaultDebugPort)
	addPort(chrome.DefaultDebugPort + 1)
	if len(ports) == 0 {
		return nil, fmt.Errorf("no tailnet debug ports configured")
	}
	var lastErr error
	for _, p := range ports {
		if !canDialHostPort(host, p, 700*time.Millisecond) {
			lastErr = fmt.Errorf("cannot dial %s:%d", host, p)
			continue
		}
		ws, err := getWebsocketURLAtHost(host, p)
		if err != nil {
			lastErr = err
			continue
		}
		ws = rewriteWebSocketHost(ws, host, p)
		session := &chrome.Session{
			PID:          0,
			Port:         p,
			WebSocketURL: ws,
			IsNew:        false,
		}
		s, err := initSession(session, strings.TrimSpace(opts.Role))
		if err != nil {
			lastErr = err
			continue
		}
		logs.Info("   [BROWSER] tailnet direct attached on %s host=%s port=%d", nodeInfo.Name, host, p)
		return s, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no attachable tailnet debug endpoint")
	}
	return nil, lastErr
}

func getWebsocketURLAtHost(host string, port int) (string, error) {
	u := fmt.Sprintf("http://%s:%d/json/version", host, port)
	resp, err := http.Get(u)
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
	ws := strings.TrimSpace(data.WebSocketDebuggerURL)
	if ws == "" {
		return "", fmt.Errorf("webSocketDebuggerUrl missing at %s", u)
	}
	return ws, nil
}

func rewriteWebSocketHost(raw, host string, port int) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil {
		return raw
	}
	if !strings.EqualFold(u.Scheme, "ws") && !strings.EqualFold(u.Scheme, "wss") {
		return raw
	}
	if p := strings.TrimSpace(u.Port()); p != "" {
		u.Host = net.JoinHostPort(host, p)
	} else {
		u.Host = net.JoinHostPort(host, strconv.Itoa(port))
	}
	return u.String()
}

func startRemoteBrowserUnixNoRepo(nodeInfo sshv1.MeshNode, opts BrowserOptions) (*BrowserSession, error) {
	url := strings.TrimSpace(opts.URL)
	if url == "" {
		url = "about:blank"
	}
	role := strings.TrimSpace(opts.Role)
	if role == "" {
		role = "test"
	}
	roleToken := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", "'", "", "\"", "").Replace(role)
	roleToken = strings.Trim(roleToken, "-")
	if roleToken == "" {
		roleToken = "test"
	}
	cfg := RuntimeConfigSnapshot()
	preferredPID := cfg.RemoteBrowserPID
	requireRole := cfg.RemoteRequireRole

	// 1) Probe existing remote debug endpoints from Chrome/Chromium listeners.
	probeScript := `
set -eu
procs="$(ps axww -o pid= -o command= | grep -Ei 'Google Chrome|google-chrome|chromium|msedge|microsoft edge' | grep -Ev 'grep|Crashpad|--type=|Helper \(Plugin\)|Helper \(Renderer\)|Helper \(GPU\)|Helper \(Alerts\)|Helper \(EH\)' || true)"
if [ -n "$procs" ]; then
  printf '%s\n' "$procs" | while IFS= read -r line; do
    [ -n "$line" ] || continue
    pid="$(printf '%s' "$line" | awk '{print $1}')"
    cmd="$(printf '%s' "$line" | cut -d' ' -f2-)"
    echo "DIALTONE_REMOTE_CHROME_PID=${pid}|${cmd}"
    arg_port="$(printf '%s' "$cmd" | sed -n 's/.*--remote-debugging-port=\([0-9][0-9]*\).*/\1/p' | head -n1)"
    listen_ports="$(lsof -nP -a -p "$pid" -iTCP -sTCP:LISTEN 2>/dev/null | awk 'NR>1 {print $9}' | sed -n 's/.*:\([0-9][0-9]*\)$/\1/p' || true)"
    ports="$(printf '%s\n%s\n' "$arg_port" "$listen_ports" | sed '/^[[:space:]]*$/d' | sort -u || true)"
    if [ -z "$ports" ]; then
      continue
    fi
    printf '%s\n' "$ports" | while IFS= read -r p; do
      [ -n "$p" ] || continue
      resp="$(curl -fsS --max-time 1 "http://127.0.0.1:${p}/json/version" 2>/dev/null || true)"
      if [ -n "$resp" ] && printf '%s' "$resp" | grep -q "webSocketDebuggerUrl"; then
        ws="$(printf '%s' "$resp" | tr -d '\n' | sed -n 's/.*"webSocketDebuggerUrl"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
        echo "DIALTONE_REMOTE_DEBUG=${pid}|${p}|${ws}"
      fi
    done
  done
fi
`
	if out, err := sshv1.RunNodeCommand(nodeInfo.Name, probeScript, sshv1.CommandOptions{}); err == nil {
		type candidate struct {
			meta chrome.SessionMetadata
			cmd  string
		}
		cmdByPID := make(map[int]string)
		candidates := make([]candidate, 0)
		procCount := 0
		debugCount := 0
		seen := make(map[string]struct{})
		sc := bufio.NewScanner(strings.NewReader(out))
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if strings.HasPrefix(line, "DIALTONE_REMOTE_CHROME_PID=") {
				procCount++
				raw := strings.TrimPrefix(line, "DIALTONE_REMOTE_CHROME_PID=")
				parts := strings.SplitN(raw, "|", 2)
				if len(parts) == 2 {
					if pid, perr := strconv.Atoi(strings.TrimSpace(parts[0])); perr == nil && pid > 0 {
						cmdByPID[pid] = strings.TrimSpace(parts[1])
					}
				}
				continue
			}
			if !strings.HasPrefix(line, "DIALTONE_REMOTE_DEBUG=") {
				continue
			}
			debugCount++
			raw := strings.TrimPrefix(line, "DIALTONE_REMOTE_DEBUG=")
			parts := strings.SplitN(raw, "|", 3)
			if len(parts) < 3 {
				continue
			}
			pid, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			port, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
			ws := strings.TrimSpace(parts[2])
			if port <= 0 || ws == "" {
				continue
			}
			key := fmt.Sprintf("%d|%d|%s", pid, port, ws)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			candidates = append(candidates, candidate{
				meta: chrome.SessionMetadata{
					PID:           pid,
					DebugPort:     port,
					WebSocketURL:  ws,
					WebSocketPath: chrome.WebSocketPathFromURL(ws),
					IsNew:         false,
				},
				cmd: strings.ToLower(strings.TrimSpace(cmdByPID[pid])),
			})
		}
		logs.Info("   [BROWSER] remote no-repo probe on %s proc_lines=%d debug_lines=%d candidates=%d", nodeInfo.Name, procCount, debugCount, len(candidates))
		candidateMatchesMode := func(c candidate) bool {
			cmd := strings.ToLower(strings.TrimSpace(c.cmd))
			if opts.Headless && !strings.Contains(cmd, "--headless") {
				return false
			}
			if !opts.Headless && strings.Contains(cmd, "--headless") {
				return false
			}
			if opts.GPU && strings.Contains(cmd, "--disable-gpu") {
				return false
			}
			return true
		}
		tryAttach := func(label string, filter func(candidate) bool) (*BrowserSession, bool) {
			attempts := 0
			filtered := make([]candidate, 0, len(candidates))
			for _, c := range candidates {
				if !candidateMatchesMode(c) {
					continue
				}
				if filter != nil && !filter(c) {
					continue
				}
				filtered = append(filtered, c)
			}
			sort.SliceStable(filtered, func(i, j int) bool {
				score := func(c candidate) int {
					s := 0
					if preferredPID > 0 && c.meta.PID == preferredPID {
						s += 200
					}
					if strings.Contains(c.cmd, "--dialtone-role="+strings.ToLower(roleToken)) {
						s += 100
					}
					if !strings.Contains(c.cmd, "--headless") {
						s += 20
					}
					if !strings.Contains(c.cmd, "--disable-gpu") {
						s += 10
					}
					if strings.Contains(c.cmd, ":5177") || strings.Contains(c.cmd, "/#hero") {
						s += 5
					}
					return s
				}
				return score(filtered[i]) > score(filtered[j])
			})
			for _, c := range filtered {
				attempts++
				if s, aerr := attachRemoteSession(nodeInfo, role, c.meta); aerr == nil {
					if opts.GPU && !opts.Headless {
						if ok := browserSupportsWebGL(s); !ok {
							logs.Warn("   [BROWSER] remote no-repo candidate lacks WebGL; skipping pid=%d port=%d", c.meta.PID, c.meta.DebugPort)
							s.Close()
							continue
						}
					}
					logs.Info("   [BROWSER] remote no-repo attached on %s via %s candidate pid=%d port=%d", nodeInfo.Name, label, c.meta.PID, c.meta.DebugPort)
					return s, true
				} else if attempts <= 3 {
					logs.Warn("   [BROWSER] remote no-repo attach candidate failed on %s via %s pid=%d port=%d err=%v", nodeInfo.Name, label, c.meta.PID, c.meta.DebugPort, aerr)
				}
			}
			return nil, false
		}
		roleNeedle := "--dialtone-role=" + strings.ToLower(roleToken)
		if preferredPID > 0 {
			if s, ok := tryAttach("preferred-pid", func(c candidate) bool {
				if c.meta.PID != preferredPID {
					return false
				}
				if requireRole && !strings.Contains(c.cmd, roleNeedle) {
					return false
				}
				return true
			}); ok {
				return s, nil
			}
		}
		if s, ok := tryAttach("role", func(c candidate) bool { return strings.Contains(c.cmd, roleNeedle) }); ok {
			return s, nil
		}
		if !requireRole {
			if s, ok := tryAttach("any", nil); ok {
				return s, nil
			}
		} else {
			logs.Warn("   [BROWSER] remote strict-role enabled on %s; skipping non-role candidate attach", nodeInfo.Name)
		}
		logs.Warn("   [BROWSER] remote no-repo probe found no attachable endpoint on %s", nodeInfo.Name)
	} else {
		logs.Warn("   [BROWSER] remote no-repo probe failed on %s: %v", nodeInfo.Name, err)
	}

	// 2) If none found, launch a fresh remote browser with debug enabled (no Dialtone dependency).
	if RuntimeConfigSnapshot().RemoteNoLaunch {
		return nil, fmt.Errorf("remote launch disabled by runtime config")
	}
	headlessFlag := ""
	if opts.Headless {
		headlessFlag = " --headless=new"
	}
	disableGPUFlag := ""
	if !opts.GPU {
		disableGPUFlag = " --disable-gpu"
	}
	launchScript := fmt.Sprintf(`
set -eu
url=%s
profile="$HOME/.dialtone-remote-%s-profile"
bin=""
for c in "google-chrome" "google-chrome-stable" "chromium-browser" "chromium" "microsoft-edge" "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"; do
  if [ -x "$c" ] || command -v "$c" >/dev/null 2>&1; then
    bin="$c"
    break
  fi
done
if [ -z "$bin" ]; then
  echo "no supported Chrome/Chromium executable found"
  exit 2
fi
port=%d
while lsof -nP -iTCP:${port} -sTCP:LISTEN >/dev/null 2>&1; do
  port=$((port+1))
done
cmd="$bin"
nohup "$cmd" --remote-debugging-port=${port} --remote-debugging-address=0.0.0.0 '--remote-allow-origins=*' --no-first-run --no-default-browser-check --user-data-dir="$profile" --new-window --dialtone-origin=true --dialtone-role=%s%s%s "$url" >/tmp/dialtone_remote_browser.log 2>&1 < /dev/null &
pid=$!
# wait for debugger endpoint
ok=0
for _ in $(seq 1 60); do
  if curl -fsS --max-time 1 "http://127.0.0.1:${port}/json/version" >/dev/null 2>&1; then
    ok=1
    break
  fi
  sleep 0.2
done
if [ "$ok" != "1" ]; then
  echo "remote browser debugger not ready"
  exit 4
fi
resp="$(curl -fsS --max-time 1 "http://127.0.0.1:${port}/json/version")"
ws="$(printf '%s' "$resp" | sed -n 's/.*"webSocketDebuggerUrl":"\([^"]*\)".*/\1/p' | head -n1)"
path="$(printf '%s' "$ws" | sed -n 's#^ws://[^/]*/\(.*\)$#/\1#p' | head -n1)"
echo "DIALTONE_CHROME_SESSION_JSON={\"pid\":${pid},\"debug_port\":${port},\"websocket_url\":\"${ws}\",\"websocket_path\":\"${path:-}\",\"is_new\":true}"
`, shellQuote(url), roleToken, chrome.DefaultDebugPort, roleToken, headlessFlag, disableGPUFlag)
	out, err := sshv1.RunNodeCommand(nodeInfo.Name, launchScript, sshv1.CommandOptions{})
	if err != nil {
		return nil, fmt.Errorf("remote no-repo browser launch on %s failed: %v output=%s", nodeInfo.Name, err, strings.TrimSpace(out))
	}
	raw := extractChromeSessionJSON(out)
	if raw == "" {
		return nil, fmt.Errorf("remote no-repo browser launch missing metadata marker")
	}
	var meta chrome.SessionMetadata
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil, fmt.Errorf("decode remote no-repo browser metadata: %w", err)
	}
	if meta.DebugPort <= 0 {
		return nil, fmt.Errorf("invalid remote no-repo debug port %d", meta.DebugPort)
	}
	return attachRemoteSession(nodeInfo, role, meta)
}

func browserSupportsWebGL(s *BrowserSession) bool {
	if s == nil {
		return false
	}
	var ok bool
	js := `(() => {
		try {
			const c = document.createElement("canvas");
			return !!(c.getContext("webgl2") || c.getContext("webgl") || c.getContext("experimental-webgl"));
		} catch (_) {
			return false;
		}
	})()`
	if err := s.RunWithTimeout(2500*time.Millisecond, chromedp.Evaluate(js, &ok)); err != nil {
		return false
	}
	return ok
}

func attachRemoteSession(nodeInfo sshv1.MeshNode, role string, meta chrome.SessionMetadata) (*BrowserSession, error) {
	wsPath := strings.TrimSpace(meta.WebSocketPath)
	if wsPath == "" {
		wsPath = chrome.WebSocketPathFromURL(meta.WebSocketURL)
	}
	if wsPath == "" {
		return nil, fmt.Errorf("remote websocket path is empty")
	}
	attachHost := strings.TrimSpace(nodeInfo.Host)
	attachPort := meta.DebugPort
	var tunnelCloser io.Closer
	noSSH := RuntimeConfigSnapshot().NoSSH
	if attachHost == "" || !canDialHostPort(attachHost, attachPort, 1500*time.Millisecond) {
		if noSSH {
			return nil, fmt.Errorf("tailnet direct attach to %s:%d unavailable (no-ssh mode)", attachHost, attachPort)
		}
		if closer, lport, err := openSSHDebugTunnel(nodeInfo, meta.DebugPort); err == nil {
			attachHost = "127.0.0.1"
			attachPort = lport
			tunnelCloser = closer
			if localWS, werr := getWebsocketURL(attachPort); werr == nil && strings.TrimSpace(localWS) != "" {
				if p := chrome.WebSocketPathFromURL(localWS); strings.TrimSpace(p) != "" {
					wsPath = p
				}
			}
		}
	}
	if strings.TrimSpace(attachHost) == "" {
		return nil, fmt.Errorf("remote attach host is empty")
	}
	session := &chrome.Session{
		PID:          meta.PID,
		Port:         attachPort,
		WebSocketURL: fmt.Sprintf("ws://%s:%d%s", attachHost, attachPort, wsPath),
		IsNew:        false,
	}
	s, err := initSession(session, role)
	if err != nil {
		if tunnelCloser != nil {
			_ = tunnelCloser.Close()
		}
		return nil, err
	}
	if tunnelCloser != nil {
		s.closers = append(s.closers, tunnelCloser)
	}
	return s, nil
}

func startRemoteBrowserWindows(nodeInfo sshv1.MeshNode, opts BrowserOptions) (*BrowserSession, error) {
	role := strings.TrimSpace(opts.Role)
	if role == "" {
		role = "test"
	}
	cfg := RuntimeConfigSnapshot()
	if cfg.NoSSH && strings.EqualFold(nodeInfo.OS, "windows") && !opts.Headless {
		logs.Warn("   [BROWSER] forcing headless mode for windows remote in no-ssh mode (role=%s)", strings.TrimSpace(opts.Role))
		opts.Headless = true
	}
	headless := "$true"
	if !opts.Headless {
		headless = "$false"
	}
	gpuDisabled := "$true"
	if opts.GPU {
		gpuDisabled = "$false"
	}
	url := strings.TrimSpace(opts.URL)
	if url == "" {
		url = "about:blank"
	}
	preferredPort := cfg.RemoteDebugPort
	if preferredPort <= 0 {
		preferredPort = chrome.DefaultDebugPort
	}
	portCandidates := normalizeRemoteDebugPorts(preferredPort, cfg.RemoteDebugPorts)
	allowPortBumpPS := "$true"
	if cfg.NoSSH {
		allowPortBumpPS = "$false"
	}
	allowLaunchPS := "$true"
	if cfg.RemoteNoLaunch {
		allowLaunchPS = "$false"
	}
	portCandidatesPS := psIntArray(portCandidates)
	ps := fmt.Sprintf(`$ErrorActionPreference='Stop'
$paths=@("$env:ProgramFiles\Google\Chrome\Application\chrome.exe","$env:ProgramFiles(x86)\Google\Chrome\Application\chrome.exe","$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe")
$exe=$null
foreach($p in $paths){ if(Test-Path $p){ $exe=$p; break } }
if(-not $exe){ Write-Error "chrome executable not found"; exit 1 }
$ports=%s
$allowPortBump=%s
$allowLaunch=%s
function Get-DialtoneDebugVersion([int]$p){
  try{
    $raw=& curl.exe -sS --max-time 1 ("http://127.0.0.1:{0}/json/version" -f $p) 2>$null
    if(-not $raw){ return $null }
    return ($raw | ConvertFrom-Json)
  }catch{
    return $null
  }
}
$port=$null
foreach($candidate in $ports){
  $v=Get-DialtoneDebugVersion([int]$candidate)
  if($v -and $v.webSocketDebuggerUrl){
    $path=([Uri]$v.webSocketDebuggerUrl).PathAndQuery
    $obj=[PSCustomObject]@{ pid=0; debug_port=$candidate; websocket_url=$v.webSocketDebuggerUrl; websocket_path=$path; debug_url=("http://127.0.0.1:{0}{1}" -f $candidate,$path); is_new=$false; generated_at_rfc3339=(Get-Date).ToUniversalTime().ToString("o") }
    $json=$obj | ConvertTo-Json -Compress
    Write-Output ("DIALTONE_CHROME_SESSION_JSON="+$json)
    exit 0
  }
  $used=Get-NetTCPConnection -State Listen -LocalPort $candidate -ErrorAction SilentlyContinue
  if(-not $used){ $port=[int]$candidate; break }
}
if(-not $allowLaunch){
  Write-Error "remote-no-launch enabled and no existing debugger found"
  exit 1
}
if($null -eq $port){
  if($allowPortBump){
    $base=[int]$ports[-1]
    for($attempt=0;$attempt -lt 20;$attempt++){
      $cand=$base+$attempt+1
      $used=Get-NetTCPConnection -State Listen -LocalPort $cand -ErrorAction SilentlyContinue
      if(-not $used){ $port=$cand; break }
    }
  }
}
if($null -eq $port){
  $port=[int]$ports[0]
}
$profile=Join-Path $env:TEMP ("dialtone-remote-%s-p"+$port)
$rule=("Dialtone Chrome DevTools "+$port)
try{
  if(-not (Get-NetFirewallRule -DisplayName $rule -ErrorAction SilentlyContinue)){
    New-NetFirewallRule -DisplayName $rule -Direction Inbound -Action Allow -Protocol TCP -LocalPort $port -Profile Any | Out-Null
  }
}catch{}
$args=@("--remote-debugging-port=$port","--remote-debugging-address=0.0.0.0","--remote-allow-origins=*","--no-first-run","--no-default-browser-check","--user-data-dir=$profile","--new-window","--dialtone-origin=true","--dialtone-role=%s")
if(%s){ $args += "--headless=new" }
if(%s){ $args += "--disable-gpu" }
$args += %s
$proc=Start-Process -FilePath $exe -ArgumentList $args -PassThru
$ws=$null
for($i=0;$i -lt 45;$i++){
  $v=Get-DialtoneDebugVersion([int]$port)
  if($v -and $v.webSocketDebuggerUrl){ $ws=$v.webSocketDebuggerUrl; break }
  Start-Sleep -Milliseconds 150
}
if(-not $ws){ Write-Error "debug websocket not ready"; exit 1 }
$stable=$true
for($j=0;$j -lt 6;$j++){
  Start-Sleep -Milliseconds 200
  $ok=Get-DialtoneDebugVersion([int]$port)
  if(-not $ok){
    $stable=$false
    break
  }
}
if(-not $stable){ Write-Error "debug websocket became unstable"; exit 1 }
$path=([Uri]$ws).PathAndQuery
$obj=[PSCustomObject]@{ pid=$proc.Id; debug_port=$port; websocket_url=$ws; websocket_path=$path; debug_url=("http://127.0.0.1:{0}{1}" -f $port,$path); is_new=$true; generated_at_rfc3339=(Get-Date).ToUniversalTime().ToString("o") }
$json=$obj | ConvertTo-Json -Compress
Write-Output ("DIALTONE_CHROME_SESSION_JSON="+$json)`, portCandidatesPS, allowPortBumpPS, allowLaunchPS, role, role, headless, gpuDisabled, psLiteral(url))

	out, err := sshv1.RunNodeCommand(nodeInfo.Name, ps, sshv1.CommandOptions{})
	if err != nil {
		return nil, fmt.Errorf("remote windows command on %s failed: %v output=%s", nodeInfo.Name, err, strings.TrimSpace(out))
	}
	raw := extractChromeSessionJSON(out)
	if raw == "" {
		return nil, fmt.Errorf("remote windows chrome session output missing metadata marker")
	}
	var meta chrome.SessionMetadata
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil, fmt.Errorf("decode remote windows chrome session metadata: %w", err)
	}
	if meta.DebugPort <= 0 {
		return nil, fmt.Errorf("invalid remote debug port %d", meta.DebugPort)
	}
	wsPath := strings.TrimSpace(meta.WebSocketPath)
	if wsPath == "" {
		wsPath = chrome.WebSocketPathFromURL(meta.WebSocketURL)
	}
	if wsPath == "" {
		return nil, fmt.Errorf("remote windows websocket path is empty")
	}
	attachHost := ""
	attachPort := meta.DebugPort
	var tunnelCloser io.Closer

	// Prefer direct TCP on host/tailnet before SSH forwarding; tunnel is fallback
	// only when transport is SSH.
	if h := resolveReachableDebugHost(meta.DebugPort, nodeInfo); h != "" {
		attachHost = h
	} else {
		if cfg.NoSSH {
			relayPort := meta.DebugPort + 10000
			if h2 := resolveReachableDebugHost(relayPort, nodeInfo); h2 != "" {
				attachHost = h2
				attachPort = relayPort
			} else if rerr := ensureWindowsDebugRelay(nodeInfo, relayPort, meta.DebugPort); rerr == nil {
				if h3 := resolveReachableDebugHost(relayPort, nodeInfo); h3 != "" {
					attachHost = h3
					attachPort = relayPort
				}
			}
			if attachHost == "" {
				return nil, fmt.Errorf("remote windows debug port %d is not reachable from this node without SSH tunnel", meta.DebugPort)
			}
		} else if client, lport, err := openSSHDebugTunnel(nodeInfo, meta.DebugPort); err == nil {
			attachHost = "127.0.0.1"
			attachPort = lport
			tunnelCloser = client
		}
		if attachHost == "" {
			return nil, fmt.Errorf("remote windows debug port %d is not reachable from this node", meta.DebugPort)
		}
	}
	if attachHost == "127.0.0.1" && attachPort > 0 {
		if localWS, err := getWebsocketURL(attachPort); err == nil && strings.TrimSpace(localWS) != "" {
			if p := chrome.WebSocketPathFromURL(localWS); strings.TrimSpace(p) != "" {
				wsPath = p
			}
		}
	}
	session := &chrome.Session{
		PID:          meta.PID,
		Port:         attachPort,
		WebSocketURL: fmt.Sprintf("ws://%s:%d%s", attachHost, attachPort, wsPath),
		IsNew:        false,
	}
	s, err := initSession(session, role)
	if err != nil {
		if tunnelCloser != nil {
			_ = tunnelCloser.Close()
		}
		return nil, err
	}
	if tunnelCloser != nil {
		s.closers = append(s.closers, tunnelCloser)
	}
	return s, nil
}

func extractChromeSessionJSON(output string) string {
	const marker = "DIALTONE_CHROME_SESSION_JSON="
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, marker) {
			return strings.TrimSpace(strings.TrimPrefix(line, marker))
		}
	}
	return ""
}

func shellQuote(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, `'`, `'\''`)
	return "'" + v + "'"
}

func psLiteral(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, `'`, `''`)
	return "'" + v + "'"
}

func psIntArray(vals []int) string {
	if len(vals) == 0 {
		return "@()"
	}
	parts := make([]string, 0, len(vals))
	for _, v := range vals {
		if v > 0 {
			parts = append(parts, strconv.Itoa(v))
		}
	}
	if len(parts) == 0 {
		return "@()"
	}
	return "@(" + strings.Join(parts, ",") + ")"
}

func normalizeRemoteDebugPorts(primary int, extras []int) []int {
	out := make([]int, 0, 1+len(extras))
	seen := map[int]struct{}{}
	add := func(v int) {
		if v <= 0 {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	add(primary)
	for _, v := range extras {
		add(v)
	}
	return out
}

func resolveReachableDebugHost(port int, nodeInfo sshv1.MeshNode) string {
	hosts := make([]string, 0, 4)
	isWindows := strings.EqualFold(strings.TrimSpace(nodeInfo.OS), "windows")
	if h := strings.TrimSpace(nodeInfo.Host); h != "" {
		if isWindows {
			if ip := sanitizeNonLoopbackIP(h); ip != "" {
				hosts = append(hosts, ip)
			} else if ip := resolveIPv4Host(h); ip != "" {
				hosts = append(hosts, ip)
			}
		} else {
			hosts = append(hosts, h)
		}
	}
	if gw := detectWSLHostGatewayIP(); gw != "" && !isWindows {
		hosts = append(hosts, gw)
	}
	// For Windows mesh nodes in WSL, localhost is often the wrong endpoint.
	// Only try it for non-Windows targets.
	if !isWindows {
		hosts = append(hosts, "127.0.0.1")
	}
	for _, h := range hosts {
		if isWindows && strings.HasPrefix(h, "127.") {
			continue
		}
		if canDialHostPort(h, port, 1200*time.Millisecond) {
			return h
		}
	}
	return ""
}

func ensureWindowsDebugRelay(nodeInfo sshv1.MeshNode, listenPort, targetPort int) error {
	if listenPort <= 0 || targetPort <= 0 {
		return fmt.Errorf("invalid relay ports listen=%d target=%d", listenPort, targetPort)
	}
	ps := fmt.Sprintf(`$ErrorActionPreference='Stop'
$listen=%d
$target=%d
netsh interface portproxy delete v4tov4 listenaddress=0.0.0.0 listenport=$listen | Out-Null
netsh interface portproxy add v4tov4 listenaddress=0.0.0.0 listenport=$listen connectaddress=127.0.0.1 connectport=$target | Out-Null
$rule=("Dialtone Chrome Relay "+$listen)
try{
  if(-not (Get-NetFirewallRule -DisplayName $rule -ErrorAction SilentlyContinue)){
    New-NetFirewallRule -DisplayName $rule -Direction Inbound -Action Allow -Protocol TCP -LocalPort $listen -Profile Any | Out-Null
  }
}catch{}
Write-Output ("relay:"+$listen+"->"+$target)`, listenPort, targetPort)
	_, err := sshv1.RunNodeCommand(nodeInfo.Name, ps, sshv1.CommandOptions{})
	return err
}

func resolveIPv4Host(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return ""
	}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			if out := sanitizeNonLoopbackIP(v4.String()); out != "" {
				return out
			}
		}
	}
	return ""
}

func sanitizeNonLoopbackIP(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsLoopback() {
		return ""
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ""
}

func detectWSLHostGatewayIP() string {
	raw, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "nameserver ") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func canDialHostPort(host string, port int, timeout time.Duration) bool {
	host = strings.TrimSpace(host)
	if host == "" || port <= 0 {
		return false
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func openSSHDebugTunnel(nodeInfo sshv1.MeshNode, remotePort int) (io.Closer, int, error) {
	client, err := sshv1.DialSSH(nodeInfo.Host, nodeInfo.Port, nodeInfo.User, "")
	if err != nil {
		if closer, port, xerr := openExternalSSHTunnel(nodeInfo, remotePort); xerr == nil {
			return closer, port, nil
		}
		return nil, 0, err
	}
	localPort, err := allocateLocalPort()
	if err != nil {
		_ = client.Close()
		return nil, 0, err
	}
	localAddr := fmt.Sprintf("127.0.0.1:%d", localPort)
	remoteAddr := fmt.Sprintf("127.0.0.1:%d", remotePort)
	if err := sshv1.ForwardRemoteToLocal(client, remoteAddr, localAddr); err != nil {
		_ = client.Close()
		return nil, 0, err
	}
	// Wait briefly for listener startup.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if canDialHostPort("127.0.0.1", localPort, 150*time.Millisecond) {
			return client, localPort, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = client.Close()
	return nil, 0, fmt.Errorf("local tunnel %s did not become ready", localAddr)
}

type processCloser struct {
	cmd *exec.Cmd
}

func (p *processCloser) Close() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	_ = p.cmd.Process.Kill()
	_, _ = p.cmd.Process.Wait()
	return nil
}

func openExternalSSHTunnel(nodeInfo sshv1.MeshNode, remotePort int) (io.Closer, int, error) {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return nil, 0, err
	}
	localPort, err := allocateLocalPort()
	if err != nil {
		return nil, 0, err
	}
	target := fmt.Sprintf("%s@%s", nodeInfo.User, nodeInfo.Host)
	localSpec := fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", localPort, remotePort)
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ConnectTimeout=6",
		"-p", nodeInfo.Port,
		"-N",
		"-L", localSpec,
		target,
	}
	cmd := exec.Command(sshPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, 0, err
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = "ssh tunnel exited early"
			}
			return nil, 0, errors.New(msg)
		}
		if canDialHostPort("127.0.0.1", localPort, 150*time.Millisecond) {
			return &processCloser{cmd: cmd}, localPort, nil
		}
		time.Sleep(60 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
	msg := strings.TrimSpace(stderr.String())
	if msg == "" {
		msg = "ssh tunnel did not become ready"
	}
	return nil, 0, errors.New(msg)
}

func allocateLocalPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok || addr.Port <= 0 {
		return 0, fmt.Errorf("failed to allocate local port")
	}
	return addr.Port, nil
}

func findAttachableDialtoneSession(role string, headless bool) *chrome.Session {
	procs, err := chrome.ListResources(true)
	if err != nil {
		return nil
	}
	for _, p := range procs {
		if p.Origin != "Dialtone" {
			continue
		}
		if strings.TrimSpace(role) != "" && p.Role != role {
			continue
		}
		if p.IsHeadless != headless {
			continue
		}
		if p.DebugPort <= 0 {
			continue
		}
		wsURL, err := getWebsocketURL(p.DebugPort)
		if err != nil || strings.TrimSpace(wsURL) == "" {
			continue
		}
		return &chrome.Session{
			PID:          p.PID,
			Port:         p.DebugPort,
			WebSocketURL: wsURL,
			IsNew:        false,
			IsWindows:    p.IsWindows,
		}
	}
	return nil
}

func (sc *StepContext) EnsureBrowser(opts BrowserOptions) (*BrowserSession, error) {
	sc.markBrowserUsed()
	if sc.Session != nil {
		if !opts.PreserveTabAndSize && !isBrowserSessionAlive(sc.Session) {
			if RemoteBrowserConfigured() {
				if rerr := waitForBrowserSessionReady(sc.Session, 4*time.Second); rerr != nil {
					return nil, fmt.Errorf("remote step browser became unavailable: %w", rerr)
				}
			} else {
				sc.Session.Close()
				sc.Session = nil
			}
		}
	}
	if sc.Session != nil {
		sc.bindBrowserSession(sc.Session)
		if err := sc.Session.EnsureOpenPage(); err != nil {
			return nil, err
		}
		if strings.TrimSpace(opts.URL) != "" && !opts.SkipNavigateOnReuse && !opts.PreserveTabAndSize {
			if err := sc.Session.Run(chromedp.Navigate(opts.URL)); err != nil {
				return nil, err
			}
		}
		if err := sc.runErrorPingCheckOnce(); err != nil {
			return nil, err
		}
		return sc.Session, nil
	}
	if sc.suiteBrowser != nil {
		if !opts.PreserveTabAndSize && !isBrowserSessionAlive(sc.suiteBrowser) {
			if RemoteBrowserConfigured() {
				if rerr := waitForBrowserSessionReady(sc.suiteBrowser, 4*time.Second); rerr != nil {
					return nil, fmt.Errorf("remote shared browser became unavailable: %w", rerr)
				}
			} else {
				sc.suiteBrowser.Close()
				sc.suiteBrowser = nil
			}
		}
	}
	if sc.suiteBrowser != nil {
		sc.bindBrowserSession(sc.suiteBrowser)
		if err := sc.Session.EnsureOpenPage(); err != nil {
			return nil, err
		}
		if strings.TrimSpace(opts.URL) != "" && !opts.SkipNavigateOnReuse && !opts.PreserveTabAndSize {
			if err := sc.Session.Run(chromedp.Navigate(opts.URL)); err != nil {
				return nil, err
			}
		}
		if err := sc.runErrorPingCheckOnce(); err != nil {
			return nil, err
		}
		return sc.Session, nil
	}
	s, err := StartBrowser(opts)
	if err != nil {
		return nil, err
	}
	sc.bindBrowserSession(s)
	if sc.setSuiteBrowser != nil {
		sc.setSuiteBrowser(s)
		sc.suiteBrowser = s
	}
	if err := s.EnsureOpenPage(); err != nil {
		return nil, err
	}
	if err := sc.runErrorPingCheckOnce(); err != nil {
		return nil, err
	}
	return s, nil
}

func isBrowserSessionAlive(s *BrowserSession) bool {
	if s == nil || s.ctx == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(s.ctx, 1200*time.Millisecond)
	defer cancel()
	var n int
	if err := chromedp.Run(ctx, chromedp.Evaluate(`1+1`, &n)); err != nil {
		return false
	}
	return n == 2
}

func waitForBrowserSessionReady(s *BrowserSession, timeout time.Duration) error {
	if s == nil {
		return fmt.Errorf("browser session is nil")
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := s.EnsureOpenPage(); err != nil {
			lastErr = err
			time.Sleep(140 * time.Millisecond)
			continue
		}
		var n int
		if err := s.RunWithTimeout(1500*time.Millisecond, chromedp.Evaluate(`1+1`, &n)); err != nil {
			lastErr = err
			time.Sleep(140 * time.Millisecond)
			continue
		}
		if n == 2 {
			return nil
		}
		lastErr = fmt.Errorf("unexpected browser eval result: %d", n)
		time.Sleep(140 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("browser did not become ready in time")
	}
	return lastErr
}

func preflightRemoteSharedBrowser(opts SuiteOptions) (*BrowserSession, error) {
	cfg := RuntimeConfigSnapshot()
	remoteNode := strings.TrimSpace(cfg.BrowserNode)
	if remoteNode == "" {
		return nil, nil
	}
	roleCandidates := []string{"test"}
	if r := strings.TrimSpace(opts.BrowserCleanupRole); r != "" {
		roleCandidates = append(roleCandidates, r)
	}
	roleCandidates = append(roleCandidates, "dev", "")
	seen := map[string]struct{}{}
	orderedRoles := make([]string, 0, len(roleCandidates))
	for _, role := range roleCandidates {
		role = strings.TrimSpace(role)
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		orderedRoles = append(orderedRoles, role)
	}
	targetURL := strings.TrimSpace(cfg.BrowserNewTargetURL)
	if targetURL == "" {
		targetURL = "about:blank"
	}
	var lastErr error
	for _, role := range orderedRoles {
		logs.Info("%s preflight browser attach on node=%s role=%q", testTag, remoteNode, role)
		s, err := StartBrowser(BrowserOptions{
			Headless:            true,
			GPU:                 false,
			Role:                role,
			ReuseExisting:       true,
			PreserveTabAndSize:  true,
			SkipNavigateOnReuse: true,
			RemoteNode:          remoteNode,
			URL:                 targetURL,
		})
		if err != nil {
			lastErr = err
			continue
		}
		if err := waitForBrowserSessionReady(s, 10*time.Second); err != nil {
			lastErr = err
			s.Close()
			continue
		}
		if cs := s.ChromeSession(); cs != nil {
			logs.Info("%s preflight browser ready pid=%d port=%d ws=%s", testTag, cs.PID, cs.Port, strings.TrimSpace(cs.WebSocketURL))
			UpdateRuntimeConfig(func(rc *RuntimeConfig) {
				if cs.Port > 0 {
					rc.RemoteDebugPort = cs.Port
				}
				if cs.PID > 0 {
					rc.RemoteBrowserPID = cs.PID
				}
				// Pin this suite run to the preflight browser instance.
				rc.RemoteNoLaunch = true
			})
		}
		return s, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no remote browser role candidates succeeded")
	}
	return nil, fmt.Errorf("remote browser preflight failed on node %s: %w", remoteNode, lastErr)
}

func (sc *StepContext) AttachBrowserByPort(port int, role string) (*BrowserSession, error) {
	sc.markBrowserUsed()
	if sc.Session != nil {
		return sc.Session, nil
	}
	s, err := ConnectToBrowser(port, role)
	if err != nil {
		return nil, err
	}
	sc.bindBrowserSession(s)
	if sc.setSuiteBrowser != nil {
		sc.setSuiteBrowser(s)
		sc.suiteBrowser = s
	}
	return s, nil
}

func (sc *StepContext) AttachBrowserByWebSocket(webSocketURL string, role string) (*BrowserSession, error) {
	sc.markBrowserUsed()
	if sc.Session != nil {
		return sc.Session, nil
	}
	session := &chrome.Session{
		PID:          0,
		Port:         0,
		WebSocketURL: strings.TrimSpace(webSocketURL),
		IsNew:        false,
	}
	s, err := initSession(session, role)
	if err != nil {
		return nil, err
	}
	sc.bindBrowserSession(s)
	if sc.setSuiteBrowser != nil {
		sc.setSuiteBrowser(s)
		sc.suiteBrowser = s
	}
	return s, nil
}

func (sc *StepContext) Browser() (*BrowserSession, error) {
	sc.markBrowserUsed()
	if sc.Session == nil {
		return nil, fmt.Errorf("browser not initialized; call EnsureBrowser or AttachBrowser first")
	}
	if err := sc.runErrorPingCheckOnce(); err != nil {
		return nil, err
	}
	return sc.Session, nil
}

func (sc *StepContext) CloseBrowser() {
	if sc.Session == nil {
		return
	}
	if sc.suiteBrowser != nil && sc.Session == sc.suiteBrowser {
		return
	}
	sc.Session.Close()
	sc.Session = nil
}

func (sc *StepContext) WaitForConsoleContains(substr string, timeout time.Duration) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	needle := strings.TrimSpace(substr)
	if needle == "" {
		return fmt.Errorf("console needle is required")
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, entry := range b.Entries() {
			if strings.Contains(entry.Text, needle) {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for browser console message containing %q", needle)
}

func (sc *StepContext) WaitForAriaLabel(label string, timeout time.Duration) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	selector := fmt.Sprintf(`[aria-label=%q]`, label)
	js := fmt.Sprintf(`(() => !!document.querySelector(%q))()`, selector)
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		var ok bool
		if err := b.Run(chromedp.Evaluate(js, &ok)); err == nil {
			if ok {
				return nil
			}
		} else {
			lastErr = err
		}
		time.Sleep(140 * time.Millisecond)
	}
	if lastErr != nil {
		return fmt.Errorf("timed out waiting for aria-label %q after %v (last error: %w)", label, timeout, lastErr)
	}
	return fmt.Errorf("timed out waiting for aria-label %q after %v", label, timeout)
}

func (sc *StepContext) ClickAriaLabel(label string) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	return b.Run(ClickAriaLabel(label))
}

func (sc *StepContext) TypeAriaLabel(label, value string) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	return b.Run(TypeAriaLabel(label, value))
}

func (sc *StepContext) PressEnterAriaLabel(label string) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	return b.Run(PressEnterAriaLabel(label))
}

func (sc *StepContext) WaitForAriaLabelAttrEquals(label, attr, expected string, timeout time.Duration) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	selector := fmt.Sprintf(`[aria-label=%q]`, label)
	js := fmt.Sprintf(`(() => {
		const el = document.querySelector(%q);
		if (!el) return false;
		return el.getAttribute(%q) === %q;
	})()`, selector, attr, expected)
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		var ok bool
		if err := b.Run(chromedp.Evaluate(js, &ok)); err == nil {
			if ok {
				return nil
			}
		} else {
			lastErr = err
		}
		time.Sleep(160 * time.Millisecond)
	}
	if lastErr != nil {
		return fmt.Errorf("timed out waiting for aria-label %q attr %q=%q after %v (last error: %w)", label, attr, expected, timeout, lastErr)
	}
	return fmt.Errorf("timed out waiting for aria-label %q attr %q=%q after %v", label, attr, expected, timeout)
}

func (sc *StepContext) ClickAriaLabelAfterWait(label string, timeout time.Duration) error {
	if err := sc.WaitForAriaLabel(label, timeout); err != nil {
		return err
	}
	return sc.ClickAriaLabel(label)
}

func (sc *StepContext) ClickAt(x, y float64) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	return b.Run(ClickAt(x, y))
}

// TapAt uses a click event as the cross-platform tap primitive for test automation.
func (sc *StepContext) TapAt(x, y float64) error {
	return sc.ClickAt(x, y)
}

func (sc *StepContext) RunBrowser(actions ...chromedp.Action) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	return b.Run(actions...)
}

func (sc *StepContext) RunBrowserWithTimeout(timeout time.Duration, actions ...chromedp.Action) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	return b.RunWithTimeout(timeout, actions...)
}

func (sc *StepContext) publishBrowserEvent(isError bool, kind, text string) {
	source := "browser"
	sc.appendBrowserLog(kind, text, isError)
	line := fmt.Sprintf("[STEP:%s] [BROWSER][%s] %s", sc.Name, strings.TrimSpace(kind), strings.TrimSpace(text))
	if isError {
		logs.ErrorFromTest(source, "%s", line)
		if sc.logger != nil {
			_ = sc.logger.ErrorfFromTest(source, "%s", line)
		}
		if sc.browserLogger != nil {
			_ = sc.browserLogger.ErrorfFromTest(source, "%s", line)
		}
		if sc.errorLogger != nil {
			_ = sc.errorLogger.ErrorfFromTest(source, "%s", line)
		}
		return
	}
	logs.InfoFromTest(source, "%s", line)
	if sc.logger != nil {
		_ = sc.logger.InfofFromTest(source, "%s", line)
	}
	if sc.browserLogger != nil {
		_ = sc.browserLogger.InfofFromTest(source, "%s", line)
	}
}

func (sc *StepContext) bindBrowserSession(s *BrowserSession) {
	if s == nil {
		return
	}
	s.onConsole = func(msg ConsoleMessage) {
		msgType := strings.ToLower(strings.TrimSpace(msg.Type))
		isErr := msgType == "error" || msgType == "assert"
		sc.publishBrowserEvent(isErr, "CONSOLE:"+msg.Type, msg.Text)
	}
	s.onError = func(msg ConsoleMessage) {
		sc.publishBrowserEvent(true, "ERROR", msg.Text)
	}
	sc.Session = s
}

func (sc *StepContext) markBrowserUsed() {
	sc.logMu.Lock()
	sc.browserUsed = true
	sc.logMu.Unlock()
}

func (sc *StepContext) BrowserWasUsed() bool {
	sc.logMu.Lock()
	defer sc.logMu.Unlock()
	return sc.browserUsed
}

func (sc *StepContext) hasBrowserActivity() bool {
	sc.logMu.Lock()
	defer sc.logMu.Unlock()
	return sc.browserUsed || len(sc.browserLogs) > 0
}

func (sc *StepContext) AutoScreenshotCaptured() bool {
	sc.logMu.Lock()
	defer sc.logMu.Unlock()
	return sc.autoShotDone
}

func (sc *StepContext) AddScreenshot(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("screenshot path is required")
	}
	sc.logMu.Lock()
	sc.stepScreenshots = append(sc.stepScreenshots, path)
	sc.logMu.Unlock()
	return nil
}

func (sc *StepContext) CaptureScreenshot(path string) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("screenshot path is required")
	}
	if dir := strings.TrimSpace(filepath.Dir(path)); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	if err := b.CaptureScreenshot(path); err != nil {
		return err
	}
	return sc.AddScreenshot(path)
}

func (sc *StepContext) snapshotStepScreenshots() []string {
	sc.logMu.Lock()
	defer sc.logMu.Unlock()
	return append([]string(nil), sc.stepScreenshots...)
}

func (sc *StepContext) ensureAutoStepScreenshot() error {
	if sc == nil || sc.Session == nil || !sc.hasBrowserActivity() {
		return nil
	}
	sc.logMu.Lock()
	if sc.autoShotDone {
		sc.logMu.Unlock()
		return nil
	}
	sc.logMu.Unlock()
	shotPath, err := captureAutoStepScreenshot(sc, sc.reportPath, sc.Name)
	if err != nil {
		return err
	}
	sc.logMu.Lock()
	sc.autoShotDone = true
	sc.stepScreenshots = append(sc.stepScreenshots, shotPath)
	sc.logMu.Unlock()
	return nil
}

type stackFrame struct {
	file string
	line int
	fn   string
}

func RunSuite(opts SuiteOptions, steps []Step) error {
	opts = normalizeSuiteReportPaths(opts)
	if opts.LogPath != "" {
		logs.Warn("%s file log path is deprecated and ignored: %s", testTag, opts.LogPath)
	}
	if opts.ErrorLogPath != "" {
		logs.Warn("%s error log path is deprecated and ignored: %s", testTag, opts.ErrorLogPath)
	}

	logs.Info("%s Starting Test Suite: %s", testTag, opts.Version)
	repoRoot := strings.TrimSpace(opts.RepoRoot)
	if repoRoot == "" {
		if cwd, err := os.Getwd(); err == nil {
			repoRoot = cwd
		}
	}

	natsURL := strings.TrimSpace(opts.NATSURL)
	if natsURL == "" {
		natsURL = "nats://127.0.0.1:4222"
	}
	natsListenURL := strings.TrimSpace(opts.NATSListenURL)
	autoStart := opts.AutoStartNATS
	nc, broker, baseSubject, natsErr := setupSuiteNATS(opts, natsURL, natsListenURL, autoStart)
	if natsErr != nil {
		logs.Warn("%s NATS suite logging disabled: %v", testTag, natsErr)
	}
	if nc != nil {
		defer nc.Close()
	}
	if broker != nil {
		defer broker.Close()
	}
	passLogger, failLogger := buildStatusLoggers(nc, baseSubject)

	startTime := time.Now()
	var results []StepResult
	var sharedBrowser *BrowserSession
	if RemoteBrowserConfigured() {
		preflightBrowser, preflightErr := preflightRemoteSharedBrowser(opts)
		if preflightErr != nil {
			return preflightErr
		}
		sharedBrowser = preflightBrowser
	}
	defer func() {
		if sharedBrowser != nil && !opts.PreserveSharedBrowser {
			sharedBrowser.Close()
		}
		// Enforce suite-end cleanup for Dialtone browser instances unless disabled by caller.
		if !opts.SkipBrowserCleanup {
			role := strings.TrimSpace(opts.BrowserCleanupRole)
			if role == "" {
				if err := chrome.KillDialtoneResources(); err != nil {
					logs.Warn("%s browser cleanup warning: %v", testTag, err)
				}
			} else {
				if err := cleanupDialtoneResourcesByRole(role); err != nil {
					logs.Warn("%s browser cleanup warning (role=%s): %v", testTag, role, err)
				}
			}
		}
	}()

	for _, step := range steps {
		timeout := step.Timeout
		if timeout == 0 {
			timeout = defaultStepTimeout
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		stepCtx := &StepContext{
			Name:            step.Name,
			Started:         time.Now(),
			SuiteSubject:    baseSubject,
			ErrorSubject:    baseSubject + ".error",
			natsURL:         natsURL,
			repoRoot:        repoRoot,
			reportPath:      opts.ReportPath,
			suiteBrowser:    sharedBrowser,
			setSuiteBrowser: func(s *BrowserSession) { sharedBrowser = s },
		}
		if sharedBrowser != nil {
			stepCtx.bindBrowserSession(sharedBrowser)
			if !RemoteBrowserConfigured() {
				if cerr := sharedBrowser.CloseExtraTabsKeepMain(); cerr != nil {
					logs.Warn("%s unable to cleanup extra browser tabs before step %s: %v", testTag, step.Name, cerr)
				}
			}
		}
		if nc != nil {
			stepSubject := baseSubject + "." + sanitizeSubjectToken(step.Name)
			browserSubject := stepSubject + ".browser"
			if stepLogger, err := logs.NewNATSLogger(nc, stepSubject); err == nil {
				stepCtx.logger = stepLogger
				stepCtx.StepSubject = stepSubject
				stepCtx.BrowserSubject = browserSubject
				if browserLogger, berr := logs.NewNATSLogger(nc, browserSubject); berr == nil {
					stepCtx.browserLogger = browserLogger
				}
				if errLogger, eerr := logs.NewNATSLogger(nc, stepCtx.ErrorSubject); eerr == nil {
					stepCtx.errorLogger = errLogger
				}
				stepCtx.passLogger = passLogger
				stepCtx.failLogger = failLogger
			}
		}

		done := make(chan struct{})
		var result StepRunResult
		var err error

		go func() {
			defer func() {
				if r := recover(); r != nil {
					frame := firstExternalFrame(3)
					if frame.file != "" {
						logs.ErrorFrom(frame.file, "%s [PROCESS][PANIC] Step %s panic: %v (%s:%d %s)", testTag, step.Name, r, frame.file, frame.line, frame.fn)
						stepCtx.TestFailf("panic: %v (%s:%d %s)", r, frame.file, frame.line, frame.fn)
					} else {
						logs.Error("%s [PROCESS][PANIC] Step %s panic: %v", testTag, step.Name, r)
						stepCtx.TestFailf("panic: %v", r)
					}
					err = fmt.Errorf("step %s panicked: %v", step.Name, r)
				}
			}()
			result, err = step.RunWithContext(stepCtx)
			close(done)
		}()

		select {
		case <-ctx.Done():
			logs.Error("%s Step %s timed out after %v", testTag, step.Name, timeout)
			err = fmt.Errorf("step %s timed out", step.Name)
			stepCtx.TestFailf("timed out after %v", timeout)
			// Try to capture a timeout screenshot if we have a session
			// We need a way to access the session here.
			// For now, let's just log.
		case <-done:
			if err != nil {
				logs.Error("%s Step %s failed: %v", testTag, step.Name, err)
				stepCtx.TestFailf("failed: %v", err)
			} else if result.Report != "" {
				logs.Info("%s Step %s report: %s", testTag, step.Name, result.Report)
				stepCtx.Logf("report: %s", result.Report)
				stepCtx.TestPassf("report: %s", result.Report)
			} else {
				stepCtx.TestPassf("completed")
			}
		}

		stepScreenshots := append([]string{}, step.Screenshots...)
		if stepCtx.hasBrowserActivity() && stepCtx.Session != nil && !stepCtx.AutoScreenshotCaptured() {
			if autoShot, shotErr := captureAutoStepScreenshot(stepCtx, opts.ReportPath, step.Name); shotErr != nil {
				logs.Warn("%s unable to capture auto screenshot for step %s: %v", testTag, step.Name, shotErr)
			} else if strings.TrimSpace(autoShot) != "" {
				stepScreenshots = append(stepScreenshots, autoShot)
			}
		}
		stepScreenshots = append(stepScreenshots, stepCtx.snapshotStepScreenshots()...)
		stepCopy := step
		stepCopy.Screenshots = dedupeStringsKeepOrder(stepScreenshots)
		stepLogs, stepErrors, browserLogs := stepCtx.snapshotStepLogs()
		results = append(results, StepResult{
			Step:        stepCopy,
			Error:       err,
			Result:      result,
			Start:       stepCtx.Started,
			End:         time.Now(),
			Logs:        stepLogs,
			Errors:      stepErrors,
			BrowserLogs: browserLogs,
		})

		if err != nil {
			break
		}
		if sharedBrowser != nil {
			if !RemoteBrowserConfigured() {
				if cerr := sharedBrowser.CloseExtraTabsKeepMain(); cerr != nil {
					logs.Warn("%s unable to cleanup extra browser tabs after step %s: %v", testTag, step.Name, cerr)
				}
			}
		}
	}

	duration := time.Since(startTime)
	logs.Info("%s Test Suite Completed in %v", testTag, duration)

	if opts.ReportPath != "" {
		if genErr := generateReports(opts, results, duration); genErr != nil {
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

func normalizeSuiteReportPaths(opts SuiteOptions) SuiteOptions {
	report := normalizeReportPathToSrcVersionDir(strings.TrimSpace(opts.ReportPath))
	if report == "" {
		if inferred := inferReportPathFromCallers(); strings.TrimSpace(inferred) != "" {
			report = inferred
		}
	}
	if report == "" {
		report = filepath.Join("test_report", "TEST.md")
	}
	opts.ReportPath = report
	raw := normalizeReportPathToSrcVersionDir(strings.TrimSpace(opts.RawReportPath))
	if raw == "" {
		ext := filepath.Ext(report)
		base := strings.TrimSuffix(report, ext)
		if ext == "" {
			raw = base + "_RAW.md"
		} else {
			raw = base + "_RAW" + ext
		}
	}
	opts.RawReportPath = raw
	return opts
}

func normalizeReportPathToSrcVersionDir(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	norm := filepath.ToSlash(path)
	m := srcVersionTestDirRe.FindStringSubmatch(norm)
	if len(m) != 2 {
		return path
	}
	root := strings.TrimSpace(m[1])
	if root == "" {
		return path
	}
	base := filepath.Base(norm)
	if base == "." || base == "/" || strings.TrimSpace(base) == "" {
		base = "TEST.md"
	}
	if strings.EqualFold(base, "test_raw.md") {
		return filepath.Join(filepath.FromSlash(root), "TEST_RAW.md")
	}
	return filepath.Join(filepath.FromSlash(root), base)
}

func inferReportPathFromCallers() string {
	pcs := make([]uintptr, 48)
	n := stdruntime.Callers(2, pcs)
	if n <= 0 {
		return ""
	}
	frames := stdruntime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		file := filepath.ToSlash(strings.TrimSpace(frame.File))
		if file != "" {
			if m := callerSrcVersionRootRe.FindStringSubmatch(file); len(m) == 2 {
				root := strings.TrimSpace(m[1])
				if root != "" {
					return filepath.Join(filepath.FromSlash(root), "TEST.md")
				}
			}
		}
		if !more {
			break
		}
	}
	return ""
}

func firstExternalFrame(skip int) stackFrame {
	pcs := make([]uintptr, 32)
	n := stdruntime.Callers(skip, pcs)
	if n <= 0 {
		return stackFrame{}
	}
	frames := stdruntime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		file := filepath.ToSlash(frame.File)
		if file != "" &&
			!strings.Contains(file, "/runtime/") &&
			!strings.Contains(file, "/plugins/test/src_v1/go/test.go") &&
			!strings.Contains(file, "/plugins/logs/src_v1/go/logger.go") &&
			!strings.Contains(file, "/plugins/logs/src_v1/go/nats.go") {
			rel := file
			if idx := strings.Index(file, "/src/"); idx >= 0 {
				rel = file[idx+1:]
			}
			return stackFrame{
				file: rel,
				line: frame.Line,
				fn:   frame.Function,
			}
		}
		if !more {
			break
		}
	}
	return stackFrame{}
}

func setupSuiteNATS(opts SuiteOptions, natsURL, natsListenURL string, autoStart bool) (*nats.Conn, *logs.EmbeddedNATS, string, error) {
	tryConnect := func(url string) (*nats.Conn, error) {
		return nats.Connect(url, nats.Timeout(1200*time.Millisecond))
	}
	nc, err := tryConnect(natsURL)
	if err != nil && autoStart {
		listenURL := strings.TrimSpace(natsListenURL)
		if listenURL == "" {
			listenURL = natsURL
		}
		broker, berr := logs.StartEmbeddedNATSOnURL(listenURL)
		if berr != nil {
			return nil, nil, "", berr
		}
		nc, err = tryConnect(natsURL)
		if err != nil {
			broker.Close()
			return nil, nil, "", fmt.Errorf("embedded nats started on %s but connect to %s failed: %w", listenURL, natsURL, err)
		}
		subj := strings.TrimSpace(opts.NATSSubject)
		if subj == "" {
			subj = "logs.test." + sanitizeSubjectToken(opts.Version)
		}
		logs.Info("%s Suite NATS logging active at %s subject=%s (embedded=true listen=%s)", testTag, natsURL, subj, listenURL)
		return nc, broker, subj, nil
	}
	if err != nil {
		return nil, nil, "", err
	}
	subj := strings.TrimSpace(opts.NATSSubject)
	if subj == "" {
		subj = "logs.test." + sanitizeSubjectToken(opts.Version)
	}
	logs.Info("%s Suite NATS logging active at %s subject=%s", testTag, natsURL, subj)
	return nc, nil, subj, nil
}

func buildStatusLoggers(nc *nats.Conn, baseSubject string) (pass *logs.NATSLogger, fail *logs.NATSLogger) {
	if nc == nil || strings.TrimSpace(baseSubject) == "" {
		return nil, nil
	}
	pass, _ = logs.NewNATSLogger(nc, baseSubject+".status.pass")
	fail, _ = logs.NewNATSLogger(nc, baseSubject+".status.fail")
	return pass, fail
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

func sanitizeFilenameToken(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "step"
	}
	s = sanitizeSubjectToken(s)
	if s == "" {
		return "step"
	}
	return s
}

func captureAutoStepScreenshot(sc *StepContext, reportPath, stepName string) (string, error) {
	if sc == nil || sc.Session == nil {
		return "", fmt.Errorf("browser session unavailable")
	}
	baseDir := strings.TrimSpace(filepath.Dir(strings.TrimSpace(reportPath)))
	if baseDir == "" || baseDir == "." {
		baseDir = "test_report"
	}
	shotDir := filepath.Join(baseDir, "screenshots")
	if err := os.MkdirAll(shotDir, 0755); err != nil {
		return "", err
	}
	shotPath := filepath.Join(shotDir, fmt.Sprintf("auto_%s.png", sanitizeFilenameToken(stepName)))
	if err := sc.Session.CaptureScreenshot(shotPath); err != nil {
		// Best-effort recovery for remote/stale-target runs.
		if recErr := sc.Session.EnsureOpenPage(); recErr != nil {
			return "", err
		}
		if retryErr := sc.Session.CaptureScreenshot(shotPath); retryErr != nil {
			return "", err
		}
	}
	return shotPath, nil
}

func dedupeStringsKeepOrder(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func cleanupDialtoneResourcesByRole(role string) error {
	procs, err := chrome.ListResources(true)
	if err != nil {
		return err
	}
	for _, p := range procs {
		if p.Origin != "Dialtone" {
			continue
		}
		if strings.TrimSpace(p.Role) != strings.TrimSpace(role) {
			continue
		}
		if killErr := chrome.KillResource(p.PID, p.IsWindows); killErr != nil {
			return killErr
		}
	}
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
