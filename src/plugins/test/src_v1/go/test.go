package test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"sync"
	"time"

	"dialtone/dev/plugins/chrome/src_v1/go"
	"dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
	"github.com/chromedp/cdproto/cdp"
	cdruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/nats-io/nats.go"
)

const testTag = "[TEST]"

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
}

func (sc *StepContext) Logf(format string, args ...any) {
	sc.Infof(format, args...)
}

func (sc *StepContext) Infof(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	source := sc.callerLocation()
	logs.InfoFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	if sc.logger != nil {
		_ = sc.logger.InfofFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	}
}

func (sc *StepContext) Warnf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	source := sc.callerLocation()
	logs.WarnFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	if sc.logger != nil {
		_ = sc.logger.WarnfFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	}
}

func (sc *StepContext) Debugf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	source := sc.callerLocation()
	logs.DebugFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	if sc.logger != nil {
		_ = sc.logger.InfofFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	}
}

func (sc *StepContext) Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
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

type StepRunResult struct {
	Report string
}

type SuiteOptions struct {
	Version        string
	RepoRoot       string
	ReportPath     string
	LogPath        string
	ErrorLogPath   string
	BrowserLogMode string
	NATSURL        string
	NATSListenURL  string
	NATSSubject    string
	AutoStartNATS  bool
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
	return chromedp.Run(s.ctx, tasks...)
}

func (s *BrowserSession) RunWithTimeout(timeout time.Duration, tasks ...chromedp.Action) error {
	if timeout <= 0 {
		return s.Run(tasks...)
	}
	ctx, cancel := context.WithTimeout(s.ctx, timeout)
	defer cancel()
	return chromedp.Run(ctx, tasks...)
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
	UserDataDir   string
	URL           string
	RemoteNode    string
	LogWriter     io.Writer
	LogPrefix     string
}

func RemoteBrowserConfigured() bool {
	return strings.TrimSpace(os.Getenv("DIALTONE_TEST_BROWSER_NODE")) != ""
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
	logs.Info("   [BROWSER] Connecting to WebSocket: %s", session.WebSocketURL)
	// Connect to the browser via websocket
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(context.Background(), session.WebSocketURL)

	// Reuse an existing page target when possible to avoid opening additional tabs.
	ctxOpts := []chromedp.ContextOption{}
	remoteAttach := strings.Contains(strings.TrimSpace(session.WebSocketURL), "ws://127.0.0.1:")
	if session.Port > 0 && !remoteAttach {
		if targetID, err := getFirstPageTargetID(session.Port); err == nil && targetID != "" {
			ctxOpts = append(ctxOpts, chromedp.WithTargetID(target.ID(targetID)))
		}
	}
	ctx, cancelCtx := chromedp.NewContext(allocCtx, ctxOpts...)

	s := &BrowserSession{
		ctx:     ctx,
		cancel:  func() { cancelCtx(); cancelAlloc() },
		Session: session,
	}

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
			logs.Info("   [BROWSER CONSOLE | PID %d] %s: %s", session.PID, ev.Type, msg.Text)
		case *cdruntime.EventExceptionThrown:
			exMsg := ConsoleMessage{
				Type: "exception",
				Text: ev.ExceptionDetails.Text,
				Time: time.Now(),
			}
			s.mu.Lock()
			s.messages = append(s.messages, exMsg)
			s.mu.Unlock()
			if s.onError != nil {
				s.onError(exMsg)
			}
			logs.Error("   [BROWSER EXCEPTION | PID %d] %s", session.PID, ev.ExceptionDetails.Text)
		}
	})

	return s, nil
}

func getFirstPageTargetID(port int) (string, error) {
	if port <= 0 {
		return "", fmt.Errorf("invalid port")
	}
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
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

func StartBrowser(opts BrowserOptions) (*BrowserSession, error) {
	if remoteNode := resolveRemoteBrowserNode(opts); remoteNode != "" {
		logs.Info("   [BROWSER] Remote node configured; trying remote-first on %s", remoteNode)
		if rs, rerr := startRemoteBrowser(remoteNode, opts); rerr == nil {
			if strings.TrimSpace(opts.URL) != "" {
				if err := chromedp.Run(rs.ctx, chromedp.Navigate(opts.URL)); err != nil {
					rs.Close()
					return nil, err
				}
			}
			return rs, nil
		}
		logs.Warn("   [BROWSER] remote-first failed on %s; falling back to local start", remoteNode)
	}

	logs.Info("   [BROWSER] Starting session (role=%s, reuse=%v, gpu=%v)...", opts.Role, opts.ReuseExisting, opts.GPU)
	session, err := chrome.StartSession(chrome.SessionOptions{
		Headless:      opts.Headless,
		GPU:           opts.GPU,
		Role:          opts.Role,
		ReuseExisting: opts.ReuseExisting,
		UserDataDir:   opts.UserDataDir,
		TargetURL:     opts.URL,
	})
	if err != nil {
		// First fallback: attach to an already-running Dialtone browser for this role/headless mode.
		if attach := findAttachableDialtoneSession(opts.Role, opts.Headless); attach != nil {
			s, aerr := initSession(attach, opts.Role)
			if aerr == nil {
				if opts.URL != "" {
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
			Headless:      opts.Headless,
			GPU:           opts.GPU,
			Role:          opts.Role,
			ReuseExisting: false,
			UserDataDir:   opts.UserDataDir,
			TargetURL:     opts.URL,
		})
		if err != nil {
			if remoteNode := resolveRemoteBrowserNode(opts); remoteNode != "" {
				logs.Warn("   [BROWSER] local launch failed; trying remote node %s", remoteNode)
				rs, rerr := startRemoteBrowser(remoteNode, opts)
				if rerr == nil {
					if strings.TrimSpace(opts.URL) != "" {
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
				if strings.TrimSpace(opts.URL) != "" {
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

	if opts.URL != "" {
		logs.Info("   [BROWSER] Navigating to: %s", opts.URL)
		if err := chromedp.Run(s.ctx, chromedp.Navigate(opts.URL)); err != nil {
			s.Close()
			if remoteNode := resolveRemoteBrowserNode(opts); remoteNode != "" {
				logs.Warn("   [BROWSER] local navigate failed; trying remote node %s", remoteNode)
				rs, rerr := startRemoteBrowser(remoteNode, opts)
				if rerr == nil {
					if strings.TrimSpace(opts.URL) != "" {
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

func resolveRemoteBrowserNode(opts BrowserOptions) string {
	if n := strings.TrimSpace(opts.RemoteNode); n != "" {
		return n
	}
	return strings.TrimSpace(os.Getenv("DIALTONE_TEST_BROWSER_NODE"))
}

func startRemoteBrowser(node string, opts BrowserOptions) (*BrowserSession, error) {
	nodeInfo, err := sshv1.ResolveMeshNode(node)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(nodeInfo.OS, "windows") {
		return startRemoteBrowserWindows(nodeInfo, opts)
	}
	role := strings.TrimSpace(opts.Role)
	if role == "" {
		role = "test"
	}
	url := strings.TrimSpace(opts.URL)
	if url == "" {
		url = "about:blank"
	}
	cmd := fmt.Sprintf("repo=''; for d in \"$HOME/dialtone\" /home/user/dialtone /home/tim/dialtone /mnt/c/Users/tim/dialtone /mnt/c/Users/timca/dialtone; do if [ -d \"$d\" ]; then repo=\"$d\"; break; fi; done; if [ -z \"$repo\" ]; then echo 'dialtone repo not found on remote node'; exit 1; fi; cd \"$repo\" && ./dialtone.sh chrome src_v1 session --role %s --headless=%t --gpu=%t --reuse-existing=%t --debug-address 0.0.0.0 --url %s",
		shellQuote(role), opts.Headless, opts.GPU, opts.ReuseExisting, shellQuote(url))
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

func startRemoteBrowserWindows(nodeInfo sshv1.MeshNode, opts BrowserOptions) (*BrowserSession, error) {
	role := strings.TrimSpace(opts.Role)
	if role == "" {
		role = "test"
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
	ps := fmt.Sprintf(`$ErrorActionPreference='Stop'
$paths=@("$env:ProgramFiles\Google\Chrome\Application\chrome.exe","$env:ProgramFiles(x86)\Google\Chrome\Application\chrome.exe","$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe")
$exe=$null
foreach($p in $paths){ if(Test-Path $p){ $exe=$p; break } }
if(-not $exe){ Write-Error "chrome executable not found"; exit 1 }
$listener=[System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Parse('127.0.0.1'),0)
$listener.Start(); $port=([System.Net.IPEndPoint]$listener.LocalEndpoint).Port; $listener.Stop()
$profile=Join-Path $env:TEMP ("dialtone-remote-%s")
$args=@("--remote-debugging-port=$port","--remote-debugging-address=0.0.0.0","--remote-allow-origins=*","--user-data-dir=$profile","--new-window","--dialtone-origin=true","--dialtone-role=%s")
if(%s){ $args += "--headless=new" }
if(%s){ $args += "--disable-gpu" }
$args += %s
$proc=Start-Process -FilePath $exe -ArgumentList $args -PassThru
$ws=$null
for($i=0;$i -lt 60;$i++){
  try{
    $v=Invoke-RestMethod -Uri ("http://127.0.0.1:{0}/json/version" -f $port) -TimeoutSec 2
    if($v.webSocketDebuggerUrl){ $ws=$v.webSocketDebuggerUrl; break }
  }catch{}
  Start-Sleep -Milliseconds 200
}
if(-not $ws){ Write-Error "debug websocket not ready"; exit 1 }
$stable=$true
for($j=0;$j -lt 6;$j++){
  Start-Sleep -Milliseconds 300
  try{
    $null=Invoke-RestMethod -Uri ("http://127.0.0.1:{0}/json/version" -f $port) -TimeoutSec 2
  }catch{
    $stable=$false
    break
  }
}
if(-not $stable){ Write-Error "debug websocket became unstable"; exit 1 }
$path=([Uri]$ws).PathAndQuery
$obj=[PSCustomObject]@{ pid=$proc.Id; debug_port=$port; websocket_url=$ws; websocket_path=$path; debug_url=("http://127.0.0.1:{0}{1}" -f $port,$path); is_new=$true; generated_at_rfc3339=(Get-Date).ToUniversalTime().ToString("o") }
$json=$obj | ConvertTo-Json -Compress
Write-Output ("DIALTONE_CHROME_SESSION_JSON="+$json)`, role, role, headless, gpuDisabled, psLiteral(url))

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

	// Prefer direct TCP on host/tailnet before SSH forwarding; tunnel is fallback.
	if h := resolveReachableDebugHost(meta.DebugPort, nodeInfo); h != "" {
		attachHost = h
	} else {
		if client, lport, err := openSSHDebugTunnel(nodeInfo, meta.DebugPort); err == nil {
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

func resolveReachableDebugHost(port int, nodeInfo sshv1.MeshNode) string {
	hosts := []string{"127.0.0.1"}
	if gw := detectWSLHostGatewayIP(); gw != "" {
		hosts = append(hosts, gw)
	}
	if h := strings.TrimSpace(nodeInfo.Host); h != "" {
		hosts = append(hosts, h)
	}
	for _, h := range hosts {
		if canDialHostPort(h, port, 1200*time.Millisecond) {
			return h
		}
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
	if sc.Session != nil {
		if !isBrowserSessionAlive(sc.Session) {
			sc.Session.Close()
			sc.Session = nil
		}
	}
	if sc.Session != nil {
		sc.bindBrowserSession(sc.Session)
		if strings.TrimSpace(opts.URL) != "" {
			if err := sc.Session.Run(chromedp.Navigate(opts.URL)); err != nil {
				return nil, err
			}
		}
		return sc.Session, nil
	}
	if sc.suiteBrowser != nil {
		if !isBrowserSessionAlive(sc.suiteBrowser) {
			sc.suiteBrowser.Close()
			sc.suiteBrowser = nil
		}
	}
	if sc.suiteBrowser != nil {
		sc.bindBrowserSession(sc.suiteBrowser)
		if strings.TrimSpace(opts.URL) != "" {
			if err := sc.Session.Run(chromedp.Navigate(opts.URL)); err != nil {
				return nil, err
			}
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

func (sc *StepContext) AttachBrowserByPort(port int, role string) (*BrowserSession, error) {
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
	if sc.Session == nil {
		return nil, fmt.Errorf("browser not initialized; call EnsureBrowser or AttachBrowser first")
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
	return b.RunWithTimeout(timeout, WaitForAriaLabel(label))
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
	return b.RunWithTimeout(timeout, WaitForAriaLabelAttrEquals(label, attr, expected, timeout))
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
	return b.Run(chromedp.MouseClickXY(x, y))
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
		sc.publishBrowserEvent(false, "CONSOLE:"+msg.Type, msg.Text)
	}
	s.onError = func(msg ConsoleMessage) {
		sc.publishBrowserEvent(true, "ERROR", msg.Text)
	}
	sc.Session = s
}

type StepResult struct {
	Step   Step
	Error  error
	Result StepRunResult
	Start  time.Time
	End    time.Time
}

type stackFrame struct {
	file string
	line int
	fn   string
}

func RunSuite(opts SuiteOptions, steps []Step) error {
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
	defer func() {
		if sharedBrowser != nil {
			sharedBrowser.Close()
		}
		// Enforce suite-end cleanup for all Dialtone-tagged browser instances.
		if err := chrome.KillDialtoneResources(); err != nil {
			logs.Warn("%s browser cleanup warning: %v", testTag, err)
		}
	}()

	for _, step := range steps {
		timeout := step.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
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
