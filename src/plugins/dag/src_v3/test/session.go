package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"dialtone/cli/src/core/browser"
	test_v2 "dialtone/cli/src/libs/test_v2"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

var sharedServer *exec.Cmd
var sharedBrowser *test_v2.BrowserSession

const (
	mobileViewportWidth  = 390
	mobileViewportHeight = 844
	mobileScaleFactor    = 2
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
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "dag", "serve", "src_v3")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	if err := test_v2.WaitForPort(8080, 12*time.Second); err != nil {
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
			URL:           "http://127.0.0.1:8080/#three",
			LogWriter:     os.Stdout,
			LogPrefix:     "[BROWSER]",
		})
		if err != nil {
			return nil, err
		}
		sharedBrowser = session
		if err := sharedBrowser.Run(chromedp.Tasks{
			chromedp.EmulateViewport(mobileViewportWidth, mobileViewportHeight, chromedp.EmulateScale(mobileScaleFactor)),
			emulation.SetDeviceMetricsOverride(mobileViewportWidth, mobileViewportHeight, mobileScaleFactor, true),
			emulation.SetTouchEmulationEnabled(true),
			chromedp.Evaluate(`window.sessionStorage.setItem('dag_test_mode', '1')`, nil),
		}); err != nil {
			return nil, err
		}
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
}
