package test

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"dialtone/dev/plugins/chrome/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/chromedp/chromedp"
	"github.com/nats-io/nats.go"
)

type DevOptions struct {
	RepoRoot          string
	PluginDir         string
	UIDir             string
	DevPort           int
	DevHost           string
	DevPublicURL      string
	Role              string
	BrowserMetaPath   string
	BrowserModeEnvVar string // e.g. "DAG_DEV_BROWSER_MODE"
	NATSURL           string
	NATSSubject       string
}

func RunDev(opts DevOptions) error {
	if opts.DevPort == 0 {
		opts.DevPort = 3000
	}
	if strings.TrimSpace(opts.DevHost) == "" {
		opts.DevHost = "127.0.0.1"
	}
	localURL := fmt.Sprintf("http://127.0.0.1:%d", opts.DevPort)
	devURL := strings.TrimSpace(opts.DevPublicURL)
	if devURL == "" {
		devURL = localURL
	}
	logOut := os.Stdout
	devLogger, _ := newDevNATSLogger(opts)
	defer func() {
		if devLogger != nil {
			devLogger.Close()
		}
	}()
	logf := func(format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		fmt.Fprintf(logOut, "%s\n", line)
		if devLogger != nil {
			_ = devLogger.Infof("%s", line)
		}
	}

	if _, err := os.Stat(opts.UIDir); os.IsNotExist(err) {
		return fmt.Errorf("UI directory not found: %s", opts.UIDir)
	}
	uiTitle, err := chrome.ReadHTMLTitle(filepath.Join(opts.UIDir, "index.html"))
	if err != nil {
		return err
	}

	if err := WaitForPort(opts.DevPort, 600*time.Millisecond); err == nil {
		matched, probeErr := chrome.DevServerMatchesVersion(opts.DevPort, uiTitle)
		if probeErr != nil {
			return fmt.Errorf("port %d is in use and could not verify existing dev server: %w", opts.DevPort, probeErr)
		}
		if !matched {
			return fmt.Errorf("port %d is already in use by a different app; stop it or choose another port", opts.DevPort)
		}

		logf("   [DEV] Dev server already running at %s", localURL)
		if strings.EqualFold(strings.TrimSpace(os.Getenv(opts.BrowserModeEnvVar)), "none") {
			logf("   [DEV] Browser launch disabled by %s=none", opts.BrowserModeEnvVar)
			logf("   [DEV] No new dev server was started.")
			return nil
		}
		logf("   [DEV] Opening dev URL in regular browser...")
		if _, err := StartDevBrowser(opts, logOut, devURL, devLogger); err != nil {
			return err
		}
		logf("   [DEV] Browser ready. No new dev server was started.")
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var (
		mu               sync.Mutex
		session          *BrowserSession
		browserBooted    bool
		restartAttemptID int
	)

	for {
		restartAttemptID++
		logf("   [DEV] Running vite dev... (attempt %d)", restartAttemptID)

		// Find bun from environment
		bunBin := filepath.Join(os.Getenv("DIALTONE_ENV"), "bun", "bin", "bun")
		if _, err := os.Stat(bunBin); err != nil {
			bunBin = "bun" // Fallback
		}

		cmd := exec.Command(bunBin, "run", "dev", "--host", opts.DevHost, "--port", strconv.Itoa(opts.DevPort), "--strictPort")
		cmd.Dir = opts.UIDir
		cmd.Stdout = logOut
		cmd.Stderr = logOut
		if err := cmd.Start(); err != nil {
			return err
		}

		go func(attempt int) {
			if err := WaitForPort(opts.DevPort, 30*time.Second); err != nil {
				logf("   [DEV] Warning: vite server not ready on port %d: %v", opts.DevPort, err)
				return
			}

			mu.Lock()
			alreadyBooted := browserBooted
			mu.Unlock()
			if alreadyBooted {
				logf("   [DEV] Vite ready at %s (attempt %d); keeping existing browser session", devURL, attempt)
				return
			}

			logf("   [DEV] Vite ready at %s", localURL)
			if strings.EqualFold(strings.TrimSpace(os.Getenv(opts.BrowserModeEnvVar)), "none") {
				logf("   [DEV] Browser launch disabled by %s=none", opts.BrowserModeEnvVar)
				return
			}
			logf("   [DEV] Opening dev URL in regular browser...")

			s, err := StartDevBrowser(opts, logOut, devURL, devLogger)
			if err != nil {
				logf("   [DEV] Warning: failed to launch debug browser: %v", err)
				return
			}
			mu.Lock()
			session = s
			browserBooted = true
			mu.Unlock()
		}(restartAttemptID)

		waitCh := make(chan error, 1)
		go func() { waitCh <- cmd.Wait() }()

		select {
		case waitErr := <-waitCh:
			if ctx.Err() != nil {
				logf("   [DEV] Stopping dev server.")
				mu.Lock()
				if session != nil {
					session.Close()
				}
				mu.Unlock()
				return nil
			}
			if waitErr != nil {
				logf("   [DEV] Vite process exited with error: %v", waitErr)
			} else {
				logf("   [DEV] Vite process exited.")
			}
			logf("   [DEV] Restarting vite in 1s...")
			time.Sleep(time.Second)
		case <-ctx.Done():
			_ = cmd.Process.Signal(os.Interrupt)
			select {
			case <-waitCh:
			case <-time.After(2 * time.Second):
				_ = cmd.Process.Kill()
				<-waitCh
			}
			logf("   [DEV] Received shutdown signal. Exiting.")
			mu.Lock()
			if session != nil {
				session.Close()
			}
			mu.Unlock()
			return nil
		}
	}
}

func StartDevBrowser(opts DevOptions, logOut io.Writer, devURL string, devLogger *devNATSLogger) (*BrowserSession, error) {
	logf := func(format string, args ...any) {
		fmt.Fprintf(logOut, format+"\n", args...)
	}

	browserMode := os.Getenv(opts.BrowserModeEnvVar)
	if browserMode == "regular" {
		if err := chrome.OpenInRegularChrome(devURL); err == nil {
			logf("   [DEV] Opened URL in regular browser profile: %s", devURL)
			return nil, nil
		}
	}

	logf("   [DEV] Starting attachable debug-profile browser session.")
	if strings.TrimSpace(RuntimeConfigSnapshot().BrowserNode) == "" {
		if err := EnsureAttachableBrowser(opts, logf, devURL); err != nil {
			return nil, err
		}
	} else {
		logf("   [DEV] Remote browser node configured; skipping local debug-profile bootstrap.")
	}

	s, err := StartBrowser(BrowserOptions{
		Headless:      false,
		GPU:           true,
		Role:          opts.Role,
		ReuseExisting: true,
		URL:           devURL,
		LogWriter:     logOut,
		LogPrefix:     "   [BROWSER]",
	})
	if err != nil {
		logf("   [DEV] Warning: reuse attach failed, launching fresh dev browser: %v", err)
		s, err = StartBrowser(BrowserOptions{
			Headless:      false,
			GPU:           true,
			Role:          opts.Role,
			ReuseExisting: false,
			URL:           devURL,
			LogWriter:     logOut,
			LogPrefix:     "   [BROWSER]",
		})
		if err != nil {
			return nil, err
		}
	}
	wireDevBrowserLogForwarding(s, devLogger, logf)

	if opts.BrowserMetaPath != "" {
		if err := chrome.WriteSessionMetadata(opts.BrowserMetaPath, s.ChromeSession()); err != nil {
			logf("   [DEV] Warning: failed to write browser metadata: %v", err)
		}
	}

	// Default mobile emulation
	if err := chromedp.Run(s.Context(), chromedp.Tasks{
		// Keep a fixed viewport only; avoid touch/mobile emulation-induced viewport shifts.
		chromedp.EmulateViewport(393, 852),
	}); err != nil {
		logf("   [DEV] Warning: failed to apply mobile emulation: %v", err)
	}

	return s, nil
}

func EnsureAttachableBrowser(opts DevOptions, logf func(string, ...any), url string) error {
	// Standard profile debug port.
	if chrome.HasReachableDevtoolsWebSocket(chrome.DefaultDebugPort) {
		return nil
	}
	if err := chrome.RelaunchProfileChromeDebug(url, opts.Role); err != nil {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if chrome.HasReachableDevtoolsWebSocket(chrome.DefaultDebugPort) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for debug browser on :%d", chrome.DefaultDebugPort)
}

type devNATSLogger struct {
	conn          *nats.Conn
	logger        *logs.NATSLogger
	browserLogger *logs.NATSLogger
	errorLogger   *logs.NATSLogger
}

func newDevNATSLogger(opts DevOptions) (*devNATSLogger, error) {
	natsURL := strings.TrimSpace(opts.NATSURL)
	if natsURL == "" {
		natsURL = "nats://127.0.0.1:4222"
	}
	subject := strings.TrimSpace(opts.NATSSubject)
	if subject == "" {
		return nil, nil
	}
	nc, err := nats.Connect(natsURL, nats.Timeout(600*time.Millisecond), nats.Name("dialtone-test-dev-logger"))
	if err != nil {
		return nil, err
	}
	logger, err := logs.NewNATSLogger(nc, subject)
	if err != nil {
		nc.Close()
		return nil, err
	}
	browserLogger, _ := logs.NewNATSLogger(nc, subject+".browser")
	errorLogger, _ := logs.NewNATSLogger(nc, subject+".error")
	return &devNATSLogger{
		conn:          nc,
		logger:        logger,
		browserLogger: browserLogger,
		errorLogger:   errorLogger,
	}, nil
}

func (d *devNATSLogger) Infof(format string, args ...any) error {
	if d == nil || d.logger == nil {
		return nil
	}
	return d.logger.InfofFrom("plugins/test/src_v1/go/dev.go", format, args...)
}

func (d *devNATSLogger) Browserf(format string, args ...any) error {
	if d == nil {
		return nil
	}
	line := fmt.Sprintf(format, args...)
	if d.logger != nil {
		_ = d.logger.InfofFrom("plugins/test/src_v1/go/dev.go", "%s", line)
	}
	if d.browserLogger == nil {
		return nil
	}
	return d.browserLogger.InfofFrom("plugins/test/src_v1/go/dev.go", "%s", line)
}

func (d *devNATSLogger) Errorf(format string, args ...any) error {
	if d == nil {
		return nil
	}
	line := fmt.Sprintf(format, args...)
	if d.logger != nil {
		_ = d.logger.ErrorfFrom("plugins/test/src_v1/go/dev.go", "%s", line)
	}
	if d.errorLogger == nil {
		return nil
	}
	return d.errorLogger.ErrorfFrom("plugins/test/src_v1/go/dev.go", "%s", line)
}

func (d *devNATSLogger) Close() {
	if d == nil || d.conn == nil {
		return
	}
	_ = d.conn.Drain()
	d.conn.Close()
}

func wireDevBrowserLogForwarding(s *BrowserSession, devLogger *devNATSLogger, logf func(string, ...any)) {
	if s == nil || devLogger == nil {
		return
	}
	s.onConsole = func(msg ConsoleMessage) {
		kind := strings.ToUpper(strings.TrimSpace(msg.Type))
		if kind == "" {
			kind = "LOG"
		}
		line := fmt.Sprintf("[BROWSER][CONSOLE:%s] %s", kind, strings.TrimSpace(msg.Text))
		_ = devLogger.Browserf("%s", line)
		if kind == "ERROR" || kind == "ASSERT" {
			_ = devLogger.Errorf("%s", line)
		}
	}
	s.onError = func(msg ConsoleMessage) {
		line := fmt.Sprintf("[BROWSER][ERROR] %s", strings.TrimSpace(msg.Text))
		_ = devLogger.Browserf("%s", line)
		_ = devLogger.Errorf("%s", line)
	}
	logf("   [DEV] Browser log forwarding enabled: %s + %s + %s", "base", "browser", "error")
}
