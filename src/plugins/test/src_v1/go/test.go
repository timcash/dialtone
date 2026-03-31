package test

import (
	"context"
	"encoding/json"
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
	"strconv"
	"strings"
	"sync"
	"time"

	chrome "dialtone/dev/plugins/chrome/src_v3"
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	"dialtone/dev/plugins/logs/src_v1/go"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	cdruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/nats-io/nats.go"
)

const testTag = "[TEST]"
const defaultStepTimeout = 10 * time.Second
const devtoolsHTTPTimeout = 3 * time.Second

var srcVersionTestDirRe = regexp.MustCompile(`(?i)(.*[/\\]src_v[0-9]+)[/\\]test(?:[/\\].*)?$`)
var callerSrcVersionRootRe = regexp.MustCompile(`(?i)(.*?/src/plugins/[^/]+/src_v[0-9]+)(?:/.*)?$`)
var devtoolsHTTPClient = &http.Client{Timeout: devtoolsHTTPTimeout}

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
	quietConsole    bool
	statusPublisher func(string, string)
	pendingStatus   []stepStatusEvent
}

type StepRunResult struct {
	Report string
}

type stepStatusEvent struct {
	Kind    string
	Message string
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
	QuietConsole          bool
}

type ConsoleMessage struct {
	Type string
	Text string
	Time time.Time
}

type BrowserSession struct {
	ctx          context.Context
	cancel       context.CancelFunc
	allocCtx     context.Context
	cancelAlloc  context.CancelFunc
	cancelTab    context.CancelFunc
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

func (s *BrowserSession) isServiceManaged() bool {
	return s != nil && s.Session != nil && strings.TrimSpace(s.Session.Host) != ""
}

func (s *BrowserSession) refreshServiceStatus() (*chrome.CommandResponse, error) {
	if !s.isServiceManaged() {
		return nil, fmt.Errorf("browser session is not service-managed")
	}
	resp, err := chrome.SendCommandByHost(s.Session.Host, chrome.CommandRequest{
		Command: "status",
		Role:    strings.TrimSpace(s.Session.Role),
	})
	if err != nil {
		return nil, err
	}
	s.applyServiceResponse(resp)
	return resp, nil
}

func (s *BrowserSession) applyServiceResponse(resp *chrome.CommandResponse) {
	if s == nil || s.Session == nil || resp == nil {
		return
	}
	s.Session.Role = strings.TrimSpace(resp.Role)
	s.Session.PID = resp.BrowserPID
	s.Session.Port = resp.ChromePort
	s.Session.NATSPort = resp.NATSPort
	s.Session.CurrentURL = strings.TrimSpace(resp.CurrentURL)
	s.Session.ManagedTarget = strings.TrimSpace(resp.ManagedTarget)
}

func (s *BrowserSession) Navigate(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return nil
	}
	if s.ctx != nil {
		if err := paceAction(s.ctx); err != nil {
			return err
		}
	}
	if s.isServiceManaged() {
		_, err := s.serviceCommand(chrome.CommandRequest{
			Command:   "goto",
			URL:       strings.TrimSpace(rawURL),
			TimeoutMS: 60000,
		})
		return err
	}
	return s.Run(chromedp.Navigate(rawURL))
}

func (s *BrowserSession) serviceCommand(req chrome.CommandRequest) (*chrome.CommandResponse, error) {
	if !s.isServiceManaged() {
		return nil, fmt.Errorf("browser session is not service-managed")
	}
	req.Role = strings.TrimSpace(s.Session.Role)
	resp, err := chrome.SendCommandByHost(s.Session.Host, req)
	if err != nil {
		resp, err = s.retryServiceCommand(req, err)
		if err != nil {
			return nil, err
		}
	}
	s.applyServiceResponse(resp)
	if len(resp.ConsoleLines) > 0 {
		s.ingestServiceConsoleLines(resp.ConsoleLines)
	}
	return resp, nil
}

func (s *BrowserSession) retryServiceCommand(req chrome.CommandRequest, prior error) (*chrome.CommandResponse, error) {
	if s == nil || !s.isServiceManaged() {
		return nil, prior
	}
	if !isRecoverableBrowserRunError(prior) {
		return nil, prior
	}
	switch strings.ToLower(strings.TrimSpace(req.Command)) {
	case "", "status", "open", "close", "reset", "shutdown":
		return nil, prior
	}
	host := strings.TrimSpace(s.Session.Host)
	role := strings.TrimSpace(s.Session.Role)
	timeoutMS := req.TimeoutMS
	if timeoutMS < 5000 {
		timeoutMS = 5000
	}
	logs.Warn("   [BROWSER] recoverable service command error (%s); resetting managed tab: %v", strings.TrimSpace(req.Command), prior)
	resetResp, resetErr := chrome.SendCommandByHost(host, chrome.CommandRequest{
		Command:   "reset",
		Role:      role,
		TimeoutMS: timeoutMS,
	})
	if resetErr == nil {
		s.applyServiceResponse(resetResp)
	} else {
		logs.Warn("   [BROWSER] service reset failed after %s error: %v", strings.TrimSpace(req.Command), resetErr)
		if recovered, rerr := waitForServiceHealthy(host, role, 5*time.Second); rerr == nil {
			s.applyServiceResponse(recovered)
		} else {
			return nil, prior
		}
	}
	resp, err := chrome.SendCommandByHost(host, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *BrowserSession) ingestServiceConsoleLines(lines []string) {
	if len(lines) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := make(map[string]struct{}, len(s.messages))
	for _, msg := range s.messages {
		seen[msg.Text] = struct{}{}
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		msg := ConsoleMessage{Type: "log", Text: line, Time: time.Now()}
		s.messages = append(s.messages, msg)
		seen[line] = struct{}{}
		if s.onConsole != nil {
			s.onConsole(msg)
		}
	}
}

func (s *BrowserSession) WaitForConsoleContains(substr string, timeout time.Duration) error {
	needle := strings.TrimSpace(substr)
	if needle == "" {
		return fmt.Errorf("console needle is required")
	}
	for _, entry := range s.Entries() {
		if strings.Contains(entry.Text, needle) {
			return nil
		}
	}
	if s.isServiceManaged() {
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		if timeout < 8*time.Second {
			timeout = 8 * time.Second
		}
		deadline := time.Now().Add(timeout)
		logs.Info("   [BROWSER] service wait-log start needle=%q timeout=%v", needle, timeout)
		waitLogTimeout := timeout
		if waitLogTimeout > 8*time.Second {
			waitLogTimeout = 8 * time.Second
		}
		resp, err := s.serviceCommand(chrome.CommandRequest{
			Command:   "wait-log",
			Contains:  needle,
			TimeoutMS: int(waitLogTimeout.Milliseconds()),
		})
		if err == nil {
			s.ingestServiceConsoleLines(resp.ConsoleLines)
			logs.Info("   [BROWSER] service wait-log matched needle=%q lines=%d", needle, len(resp.ConsoleLines))
			return nil
		}
		logs.Warn("   [BROWSER] service wait-log direct request failed: %v", err)
		attempt := 0
		for time.Now().Before(deadline) {
			attempt++
			logs.Info("   [BROWSER] service console snapshot attempt=%d needle=%q", attempt, needle)
			requestTimeout := 5 * time.Second
			if remaining := time.Until(deadline); remaining > 0 && remaining < requestTimeout {
				requestTimeout = remaining
			}
			if requestTimeout < 2500*time.Millisecond {
				requestTimeout = 2500 * time.Millisecond
			}
			resp, err := s.serviceCommand(chrome.CommandRequest{
				Command:   "console",
				TimeoutMS: int(requestTimeout.Milliseconds()),
			})
			if err != nil {
				logs.Warn("   [BROWSER] service console snapshot attempt=%d failed: %v", attempt, err)
			} else {
				lineCount := len(resp.ConsoleLines)
				entryCount := len(s.Entries())
				logs.Info("   [BROWSER] service console snapshot attempt=%d returned lines=%d entries=%d", attempt, lineCount, entryCount)
				s.ingestServiceConsoleLines(resp.ConsoleLines)
				for _, entry := range s.Entries() {
					if strings.Contains(entry.Text, needle) {
						logs.Info("   [BROWSER] service console snapshot matched needle=%q", needle)
						return nil
					}
				}
			}
			time.Sleep(250 * time.Millisecond)
		}
		return fmt.Errorf("timeout waiting for browser console message containing %q", needle)
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, entry := range s.Entries() {
			if strings.Contains(entry.Text, needle) {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for browser console message containing %q", needle)
}

func (s *BrowserSession) WaitForAriaLabel(label string, timeout time.Duration) error {
	if s.isServiceManaged() {
		_, err := s.serviceCommand(chrome.CommandRequest{
			Command:   "wait-aria",
			AriaLabel: strings.TrimSpace(label),
			TimeoutMS: int(timeout.Milliseconds()),
		})
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
		if err := s.Run(chromedp.Evaluate(js, &ok)); err == nil {
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

func (s *BrowserSession) ClickAriaLabel(label string) error {
	if s.ctx != nil {
		if err := paceAction(s.ctx); err != nil {
			return err
		}
	}
	if s.isServiceManaged() {
		_, err := s.serviceCommand(chrome.CommandRequest{
			Command:   "click-aria",
			AriaLabel: strings.TrimSpace(label),
		})
		return err
	}
	return s.Run(ClickAriaLabel(label))
}

func (s *BrowserSession) TypeAriaLabel(label, value string) error {
	if s.ctx != nil {
		if err := paceAction(s.ctx); err != nil {
			return err
		}
	}
	if s.isServiceManaged() {
		_, err := s.serviceCommand(chrome.CommandRequest{
			Command:   "type-aria",
			AriaLabel: strings.TrimSpace(label),
			Value:     value,
		})
		return err
	}
	return s.Run(TypeAriaLabel(label, value))
}

func (s *BrowserSession) PressEnterAriaLabel(label string) error {
	if s.ctx != nil {
		if err := paceAction(s.ctx); err != nil {
			return err
		}
	}
	if s.isServiceManaged() {
		_, err := s.serviceCommand(chrome.CommandRequest{
			Command:   "press-enter-aria",
			AriaLabel: strings.TrimSpace(label),
		})
		return err
	}
	return s.Run(PressEnterAriaLabel(label))
}

func (s *BrowserSession) ReadAriaLabelAttr(label, attr string) (string, error) {
	if s.isServiceManaged() {
		resp, err := s.serviceCommand(chrome.CommandRequest{
			Command:   "get-aria-attr",
			AriaLabel: strings.TrimSpace(label),
			Attr:      strings.TrimSpace(attr),
		})
		if err != nil {
			return "", err
		}
		return resp.Value, nil
	}
	selector := fmt.Sprintf(`[aria-label=%q]`, label)
	var actual string
	var ok bool
	err := s.Run(chromedp.AttributeValue(selector, attr, &actual, &ok, chromedp.ByQuery))
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("aria-label %q attr %q not found", label, attr)
	}
	return actual, nil
}

func (s *BrowserSession) SetHTML(markup string) error {
	if s == nil {
		return fmt.Errorf("browser session unavailable")
	}
	if s.ctx != nil {
		if err := paceAction(s.ctx); err != nil {
			return err
		}
	}
	if s.isServiceManaged() {
		_, err := s.serviceCommand(chrome.CommandRequest{
			Command:   "set-html",
			Value:     markup,
			TimeoutMS: 2600,
		})
		return err
	}
	if err := s.Run(chromedp.Navigate("about:blank")); err != nil {
		return err
	}
	return s.Run(chromedp.ActionFunc(func(ctx context.Context) error {
		tree, err := page.GetFrameTree().Do(ctx)
		if err != nil {
			return err
		}
		if tree == nil || tree.Frame == nil {
			return fmt.Errorf("page frame unavailable")
		}
		return page.SetDocumentContent(tree.Frame.ID, markup).Do(ctx)
	}))
}

func (s *BrowserSession) SetViewport(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("viewport width and height must be positive")
	}
	if s.ctx != nil {
		if err := paceAction(s.ctx); err != nil {
			return err
		}
	}
	if s.isServiceManaged() {
		_, err := s.serviceCommand(chrome.CommandRequest{
			Command:   "set-viewport",
			Width:     width,
			Height:    height,
			TimeoutMS: 15000,
		})
		return err
	}
	return s.Run(chromedp.EmulateViewport(int64(width), int64(height)))
}

func (s *BrowserSession) Evaluate(script string, out any) error {
	script = strings.TrimSpace(script)
	if script == "" {
		return fmt.Errorf("browser script is required")
	}
	if s.ctx != nil {
		if err := paceAction(s.ctx); err != nil {
			return err
		}
	}
	if s.isServiceManaged() {
		resp, err := s.serviceCommand(chrome.CommandRequest{
			Command:   "eval",
			Script:    script,
			TimeoutMS: 10000,
		})
		if err != nil {
			return err
		}
		if out == nil {
			return nil
		}
		raw := strings.TrimSpace(resp.Value)
		if raw == "" {
			raw = "null"
		}
		return json.Unmarshal([]byte(raw), out)
	}
	return s.Run(chromedp.Evaluate(script, out))
}

func (s *BrowserSession) WaitForAriaLabelAttrEquals(label, attr, expected string, timeout time.Duration) error {
	if s.isServiceManaged() {
		_, err := s.serviceCommand(chrome.CommandRequest{
			Command:   "wait-aria-attr",
			AriaLabel: strings.TrimSpace(label),
			Attr:      strings.TrimSpace(attr),
			Expected:  expected,
			TimeoutMS: int(timeout.Milliseconds()),
		})
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
		if err := s.Run(chromedp.Evaluate(js, &ok)); err == nil {
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

func (s *BrowserSession) Close() {
	if s.cancelTab != nil {
		s.cancelTab()
	}
	if s.cancelAlloc != nil {
		s.cancelAlloc()
	} else if s.cancel != nil {
		s.cancel()
	}
	for _, c := range s.closers {
		if c != nil {
			_ = c.Close()
		}
	}
}

func (s *BrowserSession) Run(tasks ...chromedp.Action) error {
	if s.isServiceManaged() {
		return fmt.Errorf("direct chromedp actions are unsupported with chrome src_v3 service sessions")
	}
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
	if s.isServiceManaged() {
		return fmt.Errorf("direct chromedp actions are unsupported with chrome src_v3 service sessions")
	}
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
		strings.Contains(text, "invalid context") ||
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
	if s.isServiceManaged() {
		resp, err := s.refreshServiceStatus()
		if err == nil && resp != nil && resp.BrowserPID > 0 && !resp.Unhealthy {
			return nil
		}
		resp, err = chrome.SendCommandByHost(s.Session.Host, chrome.CommandRequest{
			Command: "open",
			Role:    strings.TrimSpace(s.Session.Role),
			URL:     "about:blank",
		})
		if err != nil {
			if isRecoverableBrowserRunError(err) {
				if recovered, rerr := waitForServiceHealthy(strings.TrimSpace(s.Session.Host), strings.TrimSpace(s.Session.Role), 5*time.Second); rerr == nil {
					s.applyServiceResponse(recovered)
					return nil
				}
			}
			return err
		}
		s.applyServiceResponse(resp)
		return nil
	}
	if s == nil || s.Session == nil {
		return fmt.Errorf("browser session unavailable")
	}
	s.mu.Lock()
	allowCreate := s.allowCreate
	currentTarget := strings.TrimSpace(string(s.mainTargetID))
	s.mu.Unlock()
	if targetID, err := s.ensureFirstPageTargetIDViaCDP(allowCreate); err == nil && strings.TrimSpace(targetID) != "" {
		if currentTarget != "" && currentTarget == strings.TrimSpace(targetID) {
			return nil
		}
		return s.rebindToTarget(targetID)
	}
	ports := candidateDebugPortsForSession(s.Session)
	if len(ports) == 0 {
		return fmt.Errorf("browser session debug port unavailable")
	}
	var lastErr error
	for _, p := range ports {
		host := defaultLocalDebugHost()
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

func waitForServiceHealthy(host, role string, timeout time.Duration) (*chrome.CommandResponse, error) {
	host = strings.TrimSpace(host)
	role = strings.TrimSpace(role)
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := chrome.SendCommandByHost(host, chrome.CommandRequest{
			Command:   "status",
			Role:      role,
			TimeoutMS: 1200,
		})
		if err == nil && resp != nil && resp.BrowserPID > 0 && !resp.Unhealthy {
			return resp, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("browser service unhealthy or not running")
		}
		time.Sleep(180 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("browser service did not recover in time")
	}
	return nil, lastErr
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
	allocCtx := s.allocCtx
	cancelAlloc := s.cancelAlloc
	if allocCtx == nil || cancelAlloc == nil {
		allocCtx, cancelAlloc = chromedp.NewRemoteAllocator(context.Background(), s.Session.WebSocketURL)
		s.allocCtx = allocCtx
		s.cancelAlloc = cancelAlloc
	}
	ctxOpts := []chromedp.ContextOption{}
	if targetID != "" {
		ctxOpts = append(ctxOpts, chromedp.WithTargetID(target.ID(targetID)))
	}
	ctx, cancelCtx := chromedp.NewContext(allocCtx, ctxOpts...)
	oldCancelTab := s.cancelTab
	s.ctx = ctx
	s.cancelTab = cancelCtx
	s.cancel = func() { cancelCtx(); cancelAlloc() }
	s.mainTargetID = target.ID(targetID)
	s.attachTargetListener(ctx)
	if oldCancelTab != nil {
		oldCancelTab()
	}
	return nil
}

func (s *BrowserSession) ensureMainTargetID() error {
	if s == nil {
		return fmt.Errorf("browser session unavailable")
	}
	if s.isServiceManaged() {
		return fmt.Errorf("main target lookup is not supported for service-managed browser sessions")
	}
	if s.ctx == nil {
		return fmt.Errorf("browser context unavailable")
	}
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
	if s == nil {
		return fmt.Errorf("browser session unavailable")
	}
	if s.isServiceManaged() {
		return nil
	}
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

func (s *BrowserSession) CaptureScreenshot(path string) (err error) {
	if s.ctx != nil {
		if err := paceAction(s.ctx); err != nil {
			return err
		}
	}
	if s.isServiceManaged() {
		resp, err := s.serviceCommand(chrome.CommandRequest{
			Command:   "screenshot",
			TimeoutMS: 1200,
		})
		if err != nil {
			return err
		}
		host := ""
		if cs := s.ChromeSession(); cs != nil {
			host = strings.TrimSpace(cs.Host)
		}
		return chrome.WriteScreenshotByTarget(host, resp, path)
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("capture screenshot panic: %v", r)
		}
	}()
	var buf []byte
	if err := chromedp.Run(s.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		data, err := page.CaptureScreenshot().
			WithCaptureBeyondViewport(false).
			WithFromSurface(true).
			Do(ctx)
		if err != nil {
			return err
		}
		buf = data
		return nil
	})); err != nil {
		return err
	}
	err = os.WriteFile(path, buf, 0644)
	return err
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
	return RemoteBrowserConfigured()
}

func DirectBrowserControlAvailable() bool {
	return !RemoteBrowserConfigured()
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
	return nil, fmt.Errorf("direct browser port attachment is retired; use chrome src_v3 NATS commands through a service host")
}

func getWebsocketURL(port int) (string, error) {
	var lastErr error
	for _, host := range localDebugProbeHosts() {
		resp, err := devtoolsHTTPClient.Get(fmt.Sprintf("http://%s:%d/json/version", host, port))
		if err != nil {
			lastErr = err
			continue
		}
		var data struct {
			WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			_ = resp.Body.Close()
			lastErr = err
			continue
		}
		_ = resp.Body.Close()
		ws := strings.TrimSpace(data.WebSocketDebuggerURL)
		if ws == "" {
			lastErr = fmt.Errorf("empty websocket url for host %s port %d", host, port)
			continue
		}
		if normalized := normalizeWebSocketHost(ws, host, port); normalized != "" {
			return normalized, nil
		}
		return ws, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("failed to resolve websocket URL on port %d", port)
	}
	return "", lastErr
}

func initSession(session *chrome.Session, role string) (*BrowserSession, error) {
	if session != nil && strings.TrimSpace(session.Host) != "" && strings.TrimSpace(session.WebSocketURL) == "" {
		if strings.TrimSpace(session.Role) == "" {
			session.Role = strings.TrimSpace(role)
		}
		return &BrowserSession{
			Session:     session,
			allowCreate: true,
		}, nil
	}
	if session != nil && strings.TrimSpace(session.WebSocketURL) != "" {
		return nil, fmt.Errorf("direct websocket browser sessions are retired; use chrome src_v3 NATS service commands")
	}
	cfg := RuntimeConfigSnapshot()
	remoteMode := strings.TrimSpace(cfg.BrowserNode) != ""
	if !remoteMode && session != nil {
		if ws := strings.TrimSpace(session.WebSocketURL); ws != "" && !isLocalWebSocketHost(ws) {
			remoteMode = true
		}
	}
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
	if session != nil && session.Port > 0 && (!remoteMode || (isLocalWebSocketHost(session.WebSocketURL) && !isChromeServiceProxySession(session))) {
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
	// Always allow target creation when no page target exists. This avoids
	// dead sessions on remote/tunneled attaches that expose only the browser endpoint.
	allowCreate := true
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
		allocCtx:    allocCtx,
		cancelAlloc: cancelAlloc,
		cancelTab:   cancelCtx,
		Session:     session,
		allowCreate: allowCreate,
	}
	s.attachTargetListener(ctx)
	return s, nil
}

func isChromeServiceProxySession(session *chrome.Session) bool {
	if session == nil {
		return false
	}
	if session.Port == chrome.DefaultServicePort {
		return true
	}
	return debugPortFromWebSocketURL(session.WebSocketURL) == chrome.DefaultServicePort
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
		return defaultLocalDebugHost()
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return defaultLocalDebugHost()
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
	u.Host = net.JoinHostPort(defaultLocalDebugHost(), port)
	return u.String()
}

func localDebugProbeHosts() []string {
	if stdruntime.GOOS == "linux" {
		if gw := wslGatewayIP(); gw != "" {
			return []string{gw}
		}
	}
	return []string{"127.0.0.1"}
}

func defaultLocalDebugHost() string {
	hosts := localDebugProbeHosts()
	if len(hosts) == 0 {
		return "127.0.0.1"
	}
	return strings.TrimSpace(hosts[0])
}

func normalizeWebSocketHost(raw, host string, port int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if strings.TrimSpace(host) != "" && port > 0 {
		u.Scheme = "ws"
		u.Host = net.JoinHostPort(strings.TrimSpace(host), strconv.Itoa(port))
	}
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
	return getFirstPageTargetIDAt(defaultLocalDebugHost(), port)
}

func getFirstPageTargetIDAt(host string, port int) (string, error) {
	if port <= 0 {
		return "", fmt.Errorf("invalid port")
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = defaultLocalDebugHost()
	}
	resp, err := devtoolsHTTPClient.Get(fmt.Sprintf("http://%s/json/list", net.JoinHostPort(host, strconv.Itoa(port))))
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
	return createTargetAtPortAt(defaultLocalDebugHost(), port, url)
}

func createTargetAtPortAt(host string, port int, url string) error {
	if port <= 0 {
		return fmt.Errorf("invalid port")
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = defaultLocalDebugHost()
	}
	u := fmt.Sprintf("http://%s/json/new?%s", net.JoinHostPort(host, strconv.Itoa(port)), url)
	req, err := http.NewRequest(http.MethodPut, u, nil)
	if err != nil {
		return err
	}
	resp, err := devtoolsHTTPClient.Do(req)
	if err == nil {
		_ = resp.Body.Close()
		if resp.StatusCode < 400 {
			return nil
		}
	}
	// Fallback for older endpoints that allow GET.
	resp, err = devtoolsHTTPClient.Get(u)
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
	return ensureFirstPageTargetIDAt(defaultLocalDebugHost(), port, allowCreate)
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

func (sc *StepContext) EnsureBrowser(opts BrowserOptions) (*BrowserSession, error) {
	sc.markBrowserUsed()
	if sc.Session != nil && sc.Session.isServiceManaged() {
		sc.bindBrowserSession(sc.Session)
		if strings.TrimSpace(opts.URL) != "" && !opts.SkipNavigateOnReuse && !opts.PreserveTabAndSize {
			if err := sc.Session.Navigate(opts.URL); err != nil {
				return nil, err
			}
		}
		if err := sc.runErrorPingCheckOnce(); err != nil {
			return nil, err
		}
		return sc.Session, nil
	}
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
			if err := sc.Session.Navigate(opts.URL); err != nil {
				return nil, err
			}
		}
		if err := sc.runErrorPingCheckOnce(); err != nil {
			return nil, err
		}
		return sc.Session, nil
	}
	if sc.suiteBrowser != nil && sc.suiteBrowser.isServiceManaged() {
		sc.bindBrowserSession(sc.suiteBrowser)
		if strings.TrimSpace(opts.URL) != "" && !opts.SkipNavigateOnReuse && !opts.PreserveTabAndSize {
			if err := sc.Session.Navigate(opts.URL); err != nil {
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
			if err := sc.Session.Navigate(opts.URL); err != nil {
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
	if s != nil && s.isServiceManaged() {
		resp, err := s.refreshServiceStatus()
		return err == nil && resp != nil && resp.BrowserPID > 0 && !resp.Unhealthy
	}
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
	if s.isServiceManaged() {
		if timeout <= 0 {
			timeout = 8 * time.Second
		}
		deadline := time.Now().Add(timeout)
		var lastErr error
		for time.Now().Before(deadline) {
			resp, err := s.refreshServiceStatus()
			if err == nil && resp != nil && resp.BrowserPID > 0 && !resp.Unhealthy {
				return nil
			}
			if err != nil {
				lastErr = err
			} else {
				lastErr = fmt.Errorf("browser service unhealthy or not running")
			}
			time.Sleep(140 * time.Millisecond)
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("browser did not become ready in time")
		}
		return lastErr
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
	roleCandidates := []string{strings.TrimSpace(cfg.RemoteBrowserRole), "test"}
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
			GPU:                 true,
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
	return nil, fmt.Errorf("AttachBrowserByPort is retired; use StartBrowser/EnsureBrowser with a chrome src_v3 service host")
}

func (sc *StepContext) AttachBrowserByWebSocket(webSocketURL string, role string) (*BrowserSession, error) {
	return nil, fmt.Errorf("AttachBrowserByWebSocket is retired; use StartBrowser/EnsureBrowser with a chrome src_v3 service host")
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
	return b.WaitForConsoleContains(substr, timeout)
}

func (sc *StepContext) WaitForAriaLabel(label string, timeout time.Duration) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	return b.WaitForAriaLabel(label, timeout)
}

func (sc *StepContext) ClickAriaLabel(label string) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	return b.ClickAriaLabel(label)
}

func (sc *StepContext) TypeAriaLabel(label, value string) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	return b.TypeAriaLabel(label, value)
}

func (sc *StepContext) PressEnterAriaLabel(label string) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	return b.PressEnterAriaLabel(label)
}

func (sc *StepContext) WaitForAriaLabelAttrEquals(label, attr, expected string, timeout time.Duration) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	return b.WaitForAriaLabelAttrEquals(label, attr, expected, timeout)
}

func (sc *StepContext) ReadAriaLabelAttr(label, attr string) (string, error) {
	b, err := sc.Browser()
	if err != nil {
		return "", err
	}
	return b.ReadAriaLabelAttr(label, attr)
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

func (sc *StepContext) Goto(rawURL string) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	return b.Navigate(rawURL)
}

func (sc *StepContext) SetHTML(markup string) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	return b.SetHTML(markup)
}

func (sc *StepContext) SetViewport(width, height int) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	return b.SetViewport(width, height)
}

func (sc *StepContext) Evaluate(script string, out any) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	return b.Evaluate(script, out)
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
		// Best-effort recovery for remote/stale-target runs.
		if recErr := b.EnsureOpenPage(); recErr != nil {
			return err
		}
		if retryErr := b.CaptureScreenshot(path); retryErr != nil {
			return err
		}
	}
	if err := sc.AddScreenshot(path); err != nil {
		return err
	}
	sc.logMu.Lock()
	sc.autoShotDone = true
	sc.logMu.Unlock()
	return nil
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

	if !opts.QuietConsole {
		logs.Info("%s Starting Test Suite: %s", testTag, opts.Version)
	}
	repoRoot := strings.TrimSpace(opts.RepoRoot)
	if repoRoot == "" {
		if cwd, err := os.Getwd(); err == nil {
			repoRoot = cwd
		}
	}

	natsURL := strings.TrimSpace(opts.NATSURL)
	if natsURL == "" {
		natsURL = configv1.ResolveREPLNATSURL()
	}
	natsListenURL := strings.TrimSpace(opts.NATSListenURL)
	autoStart := opts.AutoStartNATS
	nc, broker, baseSubject, natsErr := setupSuiteNATS(opts, natsURL, natsListenURL, autoStart)
	if natsErr != nil {
		if !opts.QuietConsole {
			logs.Warn("%s NATS suite logging disabled: %v", testTag, natsErr)
		}
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
			quietConsole:    opts.QuietConsole,
			suiteBrowser:    sharedBrowser,
			setSuiteBrowser: func(s *BrowserSession) { sharedBrowser = s },
		}
		if sharedBrowser != nil {
			stepCtx.bindBrowserSession(sharedBrowser)
			if !sharedBrowser.isServiceManaged() {
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
		stepCtx.publishStatus("lifecycle", fmt.Sprintf("Starting test: %s", step.Name))

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
				if !opts.QuietConsole {
					logs.Info("%s Step %s report: %s", testTag, step.Name, result.Report)
				}
				stepCtx.Logf("report: %s", result.Report)
				stepCtx.TestPassf("report: %s", result.Report)
			} else {
				stepCtx.TestPassf("completed")
			}
		}

		stepScreenshots := append([]string{}, step.Screenshots...)
		if stepCtx.hasBrowserActivity() && stepCtx.Session != nil && !stepCtx.AutoScreenshotCaptured() {
			if stepCtx.Session.isServiceManaged() {
				logs.Info("%s skipping auto screenshot for remote-managed step %s", testTag, step.Name)
			} else {
				if autoShot, shotErr := captureAutoStepScreenshot(stepCtx, opts.ReportPath, step.Name); shotErr != nil {
					logs.Warn("%s unable to capture auto screenshot for step %s: %v", testTag, step.Name, shotErr)
				} else if strings.TrimSpace(autoShot) != "" {
					stepScreenshots = append(stepScreenshots, autoShot)
				}
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
		if sharedBrowser != nil && !sharedBrowser.isServiceManaged() {
			if cerr := sharedBrowser.CloseExtraTabsKeepMain(); cerr != nil {
				logs.Warn("%s unable to cleanup extra browser tabs after step %s: %v", testTag, step.Name, cerr)
			}
		}
	}

	duration := time.Since(startTime)
	if !opts.QuietConsole {
		logs.Info("%s Test Suite Completed in %v", testTag, duration)
	}

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
