package test

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	test_v2 "dialtone/dev/plugins/test/src_v1/go"
	wslv3 "dialtone/dev/plugins/wsl/src_v3/go"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

type testCtx struct {
	repoRoot           string
	sharedServer       *exec.Cmd
	stepCtx            *test_v2.StepContext
	baseURL            string
	webPort            int
	browserInitialized bool
}

func newTestCtx() *testCtx {
	paths, _ := wslv3.ResolvePaths("")
	repoRoot := paths.Runtime.RepoRoot
	webPort, err := test_v2.PickFreePort()
	if err != nil {
		webPort = 8080
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", webPort)
	return &testCtx{
		repoRoot: repoRoot,
		baseURL:  base,
		webPort:  webPort,
	}
}

func (t *testCtx) ensureSharedServer() error {
	if t.sharedServer != nil {
		return nil
	}

	_ = cleanupPort(t.webPort)
	buildCmd := getDialtoneCmd(t.repoRoot)
	buildCmd.Args = append(buildCmd.Args, "wsl", "src_v3", "go-build")
	buildCmd.Dir = t.repoRoot
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return err
	}

	// Use the built binary or go run
	serverCmd := getDialtoneCmd(t.repoRoot)
	serverCmd.Args = append(serverCmd.Args, "go", "src_v1", "exec", "run", "./plugins/wsl/src_v3/cmd/server/main.go")
	serverCmd.Dir = t.repoRoot
	serverCmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", t.webPort))
	serverCmd.Stdout = os.Stdout
	serverCmd.Stderr = os.Stderr
	if err := serverCmd.Start(); err != nil {
		return err
	}
	if err := t.waitHTTPReady(t.baseURL+"/api/status", 12*time.Second); err != nil {
		_ = serverCmd.Process.Kill()
		return err
	}
	t.sharedServer = serverCmd
	return nil
}

func (t *testCtx) ensureSharedBrowser() (*test_v2.BrowserSession, error) {
	if t.stepCtx == nil {
		return nil, fmt.Errorf("step context not bound")
	}

	urlArg := ""
	if !t.browserInitialized {
		urlArg = t.baseURL
	}

	_, err := t.stepCtx.EnsureBrowser(test_v2.BrowserOptions{
		Headless:      true,
		Role:          "test",
		ReuseExisting: false,
		URL:           urlArg,
	})
	if err != nil {
		return nil, err
	}
	t.browserInitialized = true

	if err := t.stepCtx.RunBrowser(
		chromedp.EmulateViewport(1280, 720),
		emulation.SetTouchEmulationEnabled(false),
	); err != nil {
		return nil, err
	}
	return t.stepCtx.Browser()
}

func (t *testCtx) teardown() {
	if t.sharedServer != nil {
		_ = t.sharedServer.Process.Kill()
		_, _ = t.sharedServer.Process.Wait()
		t.sharedServer = nil
	}
}

func (t *testCtx) bindStep(sc *test_v2.StepContext) {
	t.stepCtx = sc
}

func (t *testCtx) waitHTTPReady(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 900 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("http endpoint not ready: %s", url)
}

func (t *testCtx) navigateSection(sectionID string) error {
	return t.stepCtx.RunBrowser(chromedp.Navigate(t.baseURL + "/#" + sectionID))
}

func (t *testCtx) waitAria(label string) error {
	return t.stepCtx.WaitForAriaLabel(label, 5*time.Second)
}

func (t *testCtx) captureShot(file string) error {
	paths, _ := wslv3.ResolvePaths(t.repoRoot)
	shot := filepath.Join(paths.TestShots, file)
	b, err := t.stepCtx.Browser()
	if err != nil {
		return err
	}
	return b.CaptureScreenshot(shot)
}
