package cli

import (
	"context"
	chrome_app "dialtone/dev/plugins/chrome/app"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
)

func RunDev(versionDir string) error {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "logs", versionDir)
	uiDir := filepath.Join(cwd, "src", "plugins", "logs", versionDir, "ui")
	devLogPath := filepath.Join(pluginDir, "dev.log")
	devBrowserMetaPath := filepath.Join(pluginDir, "dev.browser.json")
	devPort := 3000
	devURL := fmt.Sprintf("http://127.0.0.1:%d", devPort)

	logFile, err := os.OpenFile(devLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open dev log at %s: %w", devLogPath, err)
	}
	defer logFile.Close()

	logOut := io.MultiWriter(os.Stdout, logFile)
	logf := func(format string, args ...any) {
		fmt.Fprintf(logOut, format+"\n", args...)
	}

	logf(">> [LOGS] Dev: %s", versionDir)
	logf("   [DEV] Writing logs to %s", devLogPath)
	logf("   [DEV] Writing browser metadata to %s", devBrowserMetaPath)

	if _, err := os.Stat(uiDir); os.IsNotExist(err) {
		return fmt.Errorf("UI directory not found: %s", uiDir)
	}
	uiTitle, err := readHTMLTitle(filepath.Join(uiDir, "index.html"))
	if err != nil {
		return err
	}

	if err := waitForPort(devPort, 600*time.Millisecond); err == nil {
		matched, probeErr := devServerMatchesVersion(devPort, uiTitle)
		if probeErr != nil {
			return fmt.Errorf("port %d is in use and could not verify existing dev server: %w", devPort, probeErr)
		}
		if !matched {
			return fmt.Errorf("port %d is already in use by a different app; stop it or choose another port", devPort)
		}

		logf("   [DEV] Dev server already running at %s", devURL)
		logf("   [DEV] Opening dev URL in regular browser...")
		if _, err := startLogsDevBrowser(logOut, devURL, devBrowserMetaPath); err != nil {
			return err
		}
		logf("   [DEV] Browser ready. No new dev server was started.")
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var (
		mu            sync.Mutex
		session       *logsDevBrowserSession
		browserBooted bool
		restartID     int
	)

	for {
		restartID++
		logf("   [DEV] Running vite dev... (attempt %d)", restartID)
		cmd := runBun(cwd, uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(devPort), "--strictPort")
		cmd.Stdout = logOut
		cmd.Stderr = logOut
		if err := cmd.Start(); err != nil {
			return err
		}

		go func(attempt int) {
			if err := waitForPort(devPort, 30*time.Second); err != nil {
				logf("   [DEV] Warning: vite server not ready on port %d: %v", devPort, err)
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

			s, err := startLogsDevBrowser(logOut, devURL, devBrowserMetaPath)
			if err != nil {
				logf("   [DEV] Warning: failed to launch debug browser: %v", err)
				return
			}
			mu.Lock()
			session = s
			browserBooted = true
			mu.Unlock()
		}(restartID)

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

type logsDevBrowserSession struct {
	session *chrome_app.Session
}

func (s *logsDevBrowserSession) Close() {
	if s == nil || s.session == nil {
		return
	}
	_ = chrome_app.CleanupSession(s.session)
}

func startLogsDevBrowser(logOut io.Writer, devURL, devBrowserMetaPath string) (*logsDevBrowserSession, error) {
	logf := func(format string, args ...any) {
		fmt.Fprintf(logOut, format+"\n", args...)
	}

	browserMode := os.Getenv("LOGS_DEV_BROWSER_MODE")
	regularMode := browserMode == "regular"
	if regularMode {
		openErr := openInRegularChrome(devURL)
		if openErr == nil {
			logf("   [DEV] Opened URL in regular browser profile: %s", devURL)
			logf("   [DEV] Skipping Dialtone-managed browser metadata/emulation in regular browser mode.")
			return nil, nil
		}
		logf("   [DEV] Could not open regular browser (%v); launching managed dev browser fallback.", openErr)
	} else {
		logf("   [DEV] Starting attachable debug-profile browser session (set LOGS_DEV_BROWSER_MODE=regular to disable).")
	}

	s, err := chrome_app.StartSession(chrome_app.SessionOptions{
		Headless:      false,
		Role:          "logs-dev",
		ReuseExisting: true,
		TargetURL:     devURL,
		GPU:           true,
	})
	if err != nil {
		logf("   [DEV] Warning: reuse attach failed, launching fresh dev browser: %v", err)
		s, err = chrome_app.StartSession(chrome_app.SessionOptions{
			Headless:      false,
			Role:          "logs-dev",
			ReuseExisting: false,
			TargetURL:     devURL,
			GPU:           true,
		})
		if err != nil {
			return nil, err
		}
	}
	if err := chrome_app.WriteSessionMetadata(devBrowserMetaPath, s); err != nil {
		logf("   [DEV] Warning: failed to write browser metadata: %v", err)
	} else if meta := chrome_app.BuildSessionMetadata(s); meta != nil {
		logf("   [DEV] Browser PID=%d", meta.PID)
	}
	logf("   [DEV] Browser session ready (PID=%d).", s.PID)
	return &logsDevBrowserSession{session: s}, nil
}

func openInRegularChrome(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("osascript", "-e", fmt.Sprintf(`tell application "Google Chrome" to open location %q`, url)).Run()
	case "linux":
		return exec.Command("xdg-open", url).Run()
	case "windows":
		return exec.Command("cmd", "/c", "start", "", "chrome", url).Run()
	default:
		return fmt.Errorf("unsupported OS for regular browser open: %s", runtime.GOOS)
	}
}
