package cli

import (
	test_v2 "dialtone/cli/src/libs/test_v2"
	chrome_app "dialtone/cli/src/plugins/chrome/app"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

func RunDev(versionDir string) error {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "dag", versionDir)
	uiDir := filepath.Join(cwd, "src", "plugins", "dag", versionDir, "ui")
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

	logf(">> [DAG] Dev: %s", versionDir)
	logf("   [DEV] Writing logs to %s", devLogPath)
	logf("   [DEV] Writing browser metadata to %s", devBrowserMetaPath)

	if _, err := os.Stat(uiDir); os.IsNotExist(err) {
		return fmt.Errorf("UI directory not found: %s", uiDir)
	}
	uiTitle, err := readHTMLTitle(filepath.Join(uiDir, "index.html"))
	if err != nil {
		return err
	}

	if err := test_v2.WaitForPort(devPort, 600*time.Millisecond); err == nil {
		matched, probeErr := devServerMatchesVersion(devPort, uiTitle)
		if probeErr != nil {
			return fmt.Errorf("port %d is in use and could not verify existing dev server: %w", devPort, probeErr)
		}
		if !matched {
			return fmt.Errorf("port %d is already in use by a different app; stop it or choose another port", devPort)
		}

		logf("   [DEV] Dev server already running at %s", devURL)
		logf("   [DEV] Opening dev URL in regular browser...")
		if _, err := startDagDevBrowser(logOut, devURL, devBrowserMetaPath); err != nil {
			return err
		}
		logf("   [DEV] Browser ready. No new dev server was started.")
		return nil
	}

	logf("   [DEV] Running vite dev...")
	cmd := runBun(cwd, uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(devPort), "--strictPort")
	cmd.Stdout = logOut
	cmd.Stderr = logOut
	if err := cmd.Start(); err != nil {
		return err
	}

	var (
		mu      sync.Mutex
		session *test_v2.BrowserSession
	)

	go func() {
		if err := test_v2.WaitForPort(devPort, 30*time.Second); err != nil {
			logf("   [DEV] Warning: vite server not ready on port %d: %v", devPort, err)
			return
		}

		logf("   [DEV] Vite ready at %s", devURL)
		logf("   [DEV] Opening dev URL in regular browser...")

		s, err := startDagDevBrowser(logOut, devURL, devBrowserMetaPath)
		if err != nil {
			logf("   [DEV] Warning: failed to launch debug browser: %v", err)
			return
		}
		mu.Lock()
		session = s
		mu.Unlock()
	}()

	err = cmd.Wait()
	if err != nil {
		logf("   [DEV] Vite process exited with error: %v", err)
	} else {
		logf("   [DEV] Vite process exited.")
	}

	mu.Lock()
	if session != nil {
		session.Close()
	}
	mu.Unlock()

	return err
}

func startDagDevBrowser(logOut io.Writer, devURL, devBrowserMetaPath string) (*test_v2.BrowserSession, error) {
	logf := func(format string, args ...any) {
		fmt.Fprintf(logOut, format+"\n", args...)
	}

	browserMode := os.Getenv("DAG_DEV_BROWSER_MODE")
	regularMode := browserMode == "regular"
	if regularMode {
		openErr := openInRegularChrome(devURL)
		if openErr == nil {
			logf("   [DEV] Opened URL in regular browser profile: %s", devURL)
			logf("   [DEV] Skipping Dialtone-managed browser metadata/emulation in regular browser mode.")
			return nil, nil
		}
		if isRegularChromeLikelyRunning() {
			return nil, fmt.Errorf("failed to attach to your running Chrome session (%v). close Chrome and rerun `./dialtone.sh dag dev src_v3` so Dialtone can launch it directly", openErr)
		}
		logf("   [DEV] Could not open regular browser (%v); launching managed dev browser fallback.", openErr)
	} else {
		logf("   [DEV] Starting attachable debug-profile browser session (set DAG_DEV_BROWSER_MODE=regular to disable).")
		if err := ensureAttachableDagDevBrowserForDev(logf, devURL); err != nil {
			return nil, err
		}
		logf("   [DEV] Debug-profile browser flow active; skipping managed browser session attach.")
		return nil, nil
	}

	s, err := test_v2.StartBrowser(test_v2.BrowserOptions{
		Headless:      false,
		Role:          "dag-dev",
		ReuseExisting: true,
		URL:           devURL,
		LogWriter:     logOut,
		LogPrefix:     "   [BROWSER]",
	})
	if err != nil {
		logf("   [DEV] Warning: reuse attach failed, launching fresh dev browser: %v", err)
		s, err = test_v2.StartBrowser(test_v2.BrowserOptions{
			Headless:      false,
			Role:          "dag-dev",
			ReuseExisting: false,
			URL:           devURL,
			LogWriter:     logOut,
			LogPrefix:     "   [BROWSER]",
		})
		if err != nil {
			return nil, err
		}
	}
	if err := chrome_app.WriteSessionMetadata(devBrowserMetaPath, s.ChromeSession()); err != nil {
		logf("   [DEV] Warning: failed to write browser metadata: %v", err)
	} else if meta := chrome_app.BuildSessionMetadata(s.ChromeSession()); meta != nil {
		logf("   [DEV] Browser PID=%d", meta.PID)
		if meta.WebSocketURL != "" {
			logf("   [DEV] Debug WS: %s", meta.WebSocketURL)
		}
		if meta.DebugURL != "" {
			logf("   [DEV] Debug URL: %s", meta.DebugURL)
		}
	}
	// iPhone 14 Pro portrait profile for live mobile-first DAG preview.
	if err := chromedp.Run(s.Context(), chromedp.Tasks{
		chromedp.EmulateViewport(393, 852, chromedp.EmulateScale(3)),
		emulation.SetDeviceMetricsOverride(393, 852, 3, true),
		emulation.SetTouchEmulationEnabled(true),
	}); err != nil {
		logf("   [DEV] Warning: failed to apply iPhone 14 Pro emulation: %v", err)
	} else {
		logf("   [DEV] Applied mobile emulation: iPhone 14 Pro (393x852 @3x)")
	}
	return s, nil
}

func ensureAttachableDagDevBrowserForDev(logf func(string, ...any), url string) error {
	if hasReachableDevtoolsWebSocket(9222) {
		logf("   [DEV] Reusing existing debug endpoint on :9222.")
		_ = openInRegularChrome(url)
		return nil
	}
	logf("   [DEV] Launching debug-profile Chrome on :9222 with dag-dev role...")
	if err := relaunchProfileChromeDebug(url); err != nil {
		return fmt.Errorf("failed to launch debug-profile Chrome: %w", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if hasReachableDevtoolsWebSocket(9222) {
			logf("   [DEV] Debug endpoint on :9222 is ready.")
			return nil
		}
		time.Sleep(400 * time.Millisecond)
	}
	logf("   [DEV] Warning: debug endpoint :9222 not observed after relaunch, opening URL in regular Chrome fallback.")
	return openInRegularChrome(url)
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

func isRegularChromeLikelyRunning() bool {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("pgrep", "-x", "Google Chrome").Run() == nil
	case "linux":
		if exec.Command("pgrep", "-x", "google-chrome").Run() == nil {
			return true
		}
		if exec.Command("pgrep", "-x", "chrome").Run() == nil {
			return true
		}
		if exec.Command("pgrep", "-x", "chromium").Run() == nil {
			return true
		}
		return false
	default:
		return false
	}
}
