package cli

import (
	test_v2 "dialtone/cli/src/libs/test_v2"
	chrome_app "dialtone/cli/src/plugins/chrome/app"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
		logf("   [DEV] Reopening/attaching debug browser...")
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
		logf("   [DEV] Launching debug browser (HEADED) with console capture...")

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

	s, err := test_v2.StartBrowser(test_v2.BrowserOptions{
		Headless:      false,
		Role:          "dev",
		ReuseExisting: true,
		URL:           devURL,
		LogWriter:     logOut,
		LogPrefix:     "   [BROWSER]",
	})
	if err != nil {
		logf("   [DEV] Warning: reuse attach failed, launching fresh dev browser: %v", err)
		s, err = test_v2.StartBrowser(test_v2.BrowserOptions{
			Headless:      false,
			Role:          "dev",
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
