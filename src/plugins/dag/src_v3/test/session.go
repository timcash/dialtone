package main

import (
	"fmt"
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
var attachMode = os.Getenv("DAG_TEST_ATTACH") == "1"
var activeAttachSession = false
var clickDelayMS = func() string {
	v := os.Getenv("DAG_TEST_CLICK_DELAY_MS")
	if v == "" {
		return "0"
	}
	return v
}()

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
			session, err = start(false, "dev", true, "http://127.0.0.1:3000/#three")
			activeAttachSession = true
		} else {
			session, err = start(true, "test", false, "http://127.0.0.1:8080/#three")
			activeAttachSession = false
		}
		if err != nil {
			return nil, err
		}
		sharedBrowser = session
		if err := sharedBrowser.Run(chromedp.Tasks{
			chromedp.EmulateViewport(mobileViewportWidth, mobileViewportHeight, chromedp.EmulateScale(mobileScaleFactor)),
			emulation.SetDeviceMetricsOverride(mobileViewportWidth, mobileViewportHeight, mobileScaleFactor, true),
			emulation.SetTouchEmulationEnabled(true),
			chromedp.Evaluate(`window.sessionStorage.setItem('dag_test_mode', '1')`, nil),
			chromedp.Evaluate(fmt.Sprintf(`window.sessionStorage.setItem('dag_test_attach', %q)`, map[bool]string{true: "1", false: "0"}[activeAttachSession]), nil),
			chromedp.Evaluate(fmt.Sprintf(`window.sessionStorage.setItem('dag_test_click_delay_ms', %q)`, clickDelayMS), nil),
		}); err != nil {
			return nil, err
		}
	}

	if emitProofOfLife {
		_ = sharedBrowser.Run(chromedp.Evaluate(`console.error('[PROOFOFLIFE] Intentional Browser Test Error')`, nil))
	}

	return sharedBrowser, nil
}

type evalResult struct {
	OK  bool   `json:"ok"`
	Msg string `json:"msg"`
}

func runThreeCase(browser *test_v2.BrowserSession, name string) error {
	if browser == nil {
		return fmt.Errorf("missing browser session")
	}
	var result evalResult
	if err := browser.Run(chromedp.Evaluate(
		fmt.Sprintf(`(() => {
			const lib = window.dagTestLib;
			if (!lib || typeof lib.run !== 'function') return { ok: false, msg: 'dagTestLib is not available' };
			return lib.run(%q);
		})()`, name),
		&result,
	)); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("%s failed: %s", name, result.Msg)
	}
	if activeAttachSession {
		time.Sleep(250 * time.Millisecond)
	}
	return nil
}

func screenshotPath(file string) (string, error) {
	repoRoot, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(repoRoot, "src", "plugins", "dag", "src_v3", "screenshots", file), nil
}

func captureStoryShot(browser *test_v2.BrowserSession, file string) error {
	shot, err := screenshotPath(file)
	if err != nil {
		return err
	}
	return browser.CaptureScreenshot(shot)
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
