package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"dialtone/dev/browser"
	test_v2 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/chromedp/chromedp"
)

var sharedServer *exec.Cmd
var sharedBrowser *test_v2.BrowserSession

const (
	testViewportWidth  = 390
	testViewportHeight = 844
)

func ensureSharedServer() error {
	if sharedServer != nil {
		return nil
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	_ = browser.CleanupPort(8080)

	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "cloudflare", "serve", "src_v1")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	if err := waitForPort("127.0.0.1:8080", 12*time.Second); err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return err
	}

	sharedServer = cmd
	return nil
}

func ensureSharedBrowser(emitProofOfLife bool) (*test_v2.BrowserSession, error) {
	if err := ensureSharedServer(); err != nil {
		return nil, err
	}

	if sharedBrowser == nil {
		session, err := test_v2.StartBrowser(test_v2.BrowserOptions{
			Headless:      true,
			Role:          "test",
			ReuseExisting: false,
			URL:           "http://127.0.0.1:8080",
			LogWriter:     os.Stdout,
			LogPrefix:     "[BROWSER]",
		})
		if err != nil {
			return nil, err
		}
		if err := session.Run(chromedp.EmulateViewport(testViewportWidth, testViewportHeight)); err != nil {
			session.Close()
			return nil, err
		}
		sharedBrowser = session
	}

	if emitProofOfLife {
		_ = sharedBrowser.Run(chromedp.Evaluate(`console.error('[PROOFOFLIFE] Intentional Browser Test Error')`, nil))
	}

	return sharedBrowser, nil
}

func teardownSharedEnv() {
	if sharedBrowser != nil {
		sharedBrowser.Close()
		sharedBrowser = nil
	}
	if sharedServer != nil {
		_ = sharedServer.Process.Kill()
		_, _ = sharedServer.Process.Wait()
		sharedServer = nil
	}
	_ = browser.CleanupPort(8080)
}
