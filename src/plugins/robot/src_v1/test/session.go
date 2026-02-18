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
var attachMode = os.Getenv("ROBOT_TEST_ATTACH") == "1"
var activeAttachSession = false

const (
	testViewportWidth  = 390
	testViewportHeight = 844
	testScaleFactor    = 2
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

	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "robot", "start", "--mock", "--local-only", "--web-port", "8080", "--hostname", "robot-test-client")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	if err := test_v2.WaitForPort(8080, 15*time.Second); err != nil {
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
		start := func(headless bool, role string, reuse bool, url string) (*test_v2.BrowserSession, error) {
			return test_v2.StartBrowser(test_v2.BrowserOptions{
				Headless:      headless,
				Role:          role,
				ReuseExisting: reuse,
				URL:           url,
				LogWriter:     nil,
				LogPrefix:     "[BROWSER]",
			})
		}

		var (
			session *test_v2.BrowserSession
			err     error
		)

		if attachMode {
			session, err = start(false, "dev", true, "http://127.0.0.1:3000")
			activeAttachSession = true
		} else {
			session, err = start(true, "test", false, "http://127.0.0.1:8080?test=true")
			activeAttachSession = false
		}

		if err != nil {
			return nil, err
		}
		sharedBrowser = session

		if err := sharedBrowser.Run(chromedp.Tasks{
			chromedp.EmulateViewport(testViewportWidth, testViewportHeight, chromedp.EmulateScale(testScaleFactor)),
			emulation.SetDeviceMetricsOverride(testViewportWidth, testViewportHeight, testScaleFactor, true),
			emulation.SetTouchEmulationEnabled(true),
		}); err != nil {
			sharedBrowser.Close()
			sharedBrowser = nil
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
		if !activeAttachSession {
			sharedBrowser.Close()
		}
		sharedBrowser = nil
	}
	if sharedServer != nil {
		_ = sharedServer.Process.Kill()
		_, _ = sharedServer.Process.Wait()
		sharedServer = nil
	}
}

func captureStoryShot(browser *test_v2.BrowserSession, file string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	shot := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v1", "test", "screenshots", file)
	_ = os.MkdirAll(filepath.Dir(shot), 0755)
	return browser.CaptureScreenshot(shot)
}
