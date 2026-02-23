package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	cloudflarev1 "dialtone/dev/plugins/cloudflare/src_v1/go"
	test_v2 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/chromedp/chromedp"
)

var sharedServer *exec.Cmd
var sharedBrowser *test_v2.BrowserSession

const (
	testViewportWidth  = 390
	testViewportHeight = 844
	testServerPort     = 18080
)

func ensureSharedServer() error {
	if sharedServer != nil {
		return nil
	}

	paths, err := cloudflarev1.ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}

	_ = chrome.CleanupPort(testServerPort)

	cmd := exec.Command(filepath.Join(paths.Runtime.RepoRoot, "dialtone.sh"), "cloudflare", "src_v1", "serve")
	cmd.Dir = paths.Runtime.RepoRoot
	cmd.Env = append(os.Environ(), fmt.Sprintf("CLOUDFLARE_PORT=%d", testServerPort))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	if err := waitForPort(fmt.Sprintf("127.0.0.1:%d", testServerPort), 12*time.Second); err != nil {
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
			URL:           fmt.Sprintf("http://127.0.0.1:%d", testServerPort),
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
	_ = chrome.CleanupPort(testServerPort)
}

func testRepoRoot() (string, error) {
	paths, err := cloudflarev1.ResolvePaths("", "src_v1")
	if err != nil {
		return "", err
	}
	return paths.Runtime.RepoRoot, nil
}

func screenshotPath(name string) (string, error) {
	paths, err := cloudflarev1.ResolvePaths("", "src_v1")
	if err != nil {
		return "", err
	}
	return filepath.Join(paths.PluginVersionRoot, "screenshots", name), nil
}

func navigateToSection(session *test_v2.BrowserSession, sectionID string) error {
	return session.Run(chromedp.Evaluate(fmt.Sprintf(`window.location.hash = %q;`, sectionID), nil))
}
