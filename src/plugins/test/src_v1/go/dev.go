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
	"sync"
	"syscall"
	"time"

	"dialtone/dev/plugins/chrome/app"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

type DevOptions struct {
	RepoRoot           string
	PluginDir          string
	UIDir              string
	DevPort            int
	Role               string
	BrowserMetaPath    string
	BrowserModeEnvVar  string // e.g. "DAG_DEV_BROWSER_MODE"
}

func RunDev(opts DevOptions) error {
	if opts.DevPort == 0 {
		opts.DevPort = 3000
	}
	devURL := fmt.Sprintf("http://127.0.0.1:%d", opts.DevPort)
	devLogPath := filepath.Join(opts.PluginDir, "dev.log")

	logFile, err := os.OpenFile(devLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open dev log at %s: %w", devLogPath, err)
	}
	defer logFile.Close()

	logOut := io.MultiWriter(os.Stdout, logFile)
	logf := func(format string, args ...any) {
		fmt.Fprintf(logOut, format+"\n", args...)
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

		logf("   [DEV] Dev server already running at %s", devURL)
		logf("   [DEV] Opening dev URL in regular browser...")
		if _, err := StartDevBrowser(opts, logOut, devURL); err != nil {
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

		cmd := exec.Command(bunBin, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(opts.DevPort), "--strictPort")
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

			logf("   [DEV] Vite ready at %s", devURL)
			logf("   [DEV] Opening dev URL in regular browser...")

			s, err := StartDevBrowser(opts, logOut, devURL)
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

func StartDevBrowser(opts DevOptions, logOut io.Writer, devURL string) (*BrowserSession, error) {
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
	if err := EnsureAttachableBrowser(opts, logf, devURL); err != nil {
		return nil, err
	}

	s, err := StartBrowser(BrowserOptions{
		Headless:      false,
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

	if opts.BrowserMetaPath != "" {
		if err := chrome.WriteSessionMetadata(opts.BrowserMetaPath, s.ChromeSession()); err != nil {
			logf("   [DEV] Warning: failed to write browser metadata: %v", err)
		}
	}

	// Default mobile emulation
	if err := chromedp.Run(s.Context(), chromedp.Tasks{
		chromedp.EmulateViewport(393, 852, chromedp.EmulateScale(3)),
		emulation.SetDeviceMetricsOverride(393, 852, 3, true),
		emulation.SetTouchEmulationEnabled(true),
	}); err != nil {
		logf("   [DEV] Warning: failed to apply mobile emulation: %v", err)
	}

	return s, nil
}

func EnsureAttachableBrowser(opts DevOptions, logf func(string, ...any), url string) error {
	// Standard port 9222 for profile-based debug
	if chrome.HasReachableDevtoolsWebSocket(9222) {
		return nil
	}
	if err := chrome.RelaunchProfileChromeDebug(url, opts.Role); err != nil {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if chrome.HasReachableDevtoolsWebSocket(9222) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for debug browser on :9222")
}
