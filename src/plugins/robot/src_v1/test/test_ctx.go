package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dialtone/dev/browser"
	test_v2 "dialtone/dev/plugins/dag/src_v3/suite"
	chrome_app "dialtone/dev/plugins/chrome/app"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

type testCtx struct {
	sharedServer        *exec.Cmd
	sharedBrowser       *test_v2.BrowserSession
	attachMode          bool
	activeAttachSession bool
	requireBackend      bool
	keepViewport        bool
	baseURL             string
	devBaseURL          string
	clickGap            time.Duration
	lastClickAt         time.Time
}

func newTestCtx() *testCtx {
	attach := os.Getenv("ROBOT_TEST_ATTACH") == "1"
	keepViewport := strings.TrimSpace(os.Getenv("ROBOT_TEST_KEEP_VIEWPORT")) == "1"
	if attach && !keepViewport {
		keepViewport = true
	}
	base := strings.TrimSpace(os.Getenv("ROBOT_TEST_BASE_URL"))
	devBase := strings.TrimSpace(os.Getenv("ROBOT_TEST_DEV_BASE_URL"))
	cpsRaw := strings.TrimSpace(os.Getenv("ROBOT_TEST_CPS"))
	cps := 3
	if cpsRaw != "" {
		if parsed, err := strconv.Atoi(cpsRaw); err == nil && parsed >= 1 {
			cps = parsed
		}
	}
	if base == "" {
		if attach {
			base = "http://127.0.0.1:3000"
		} else {
			base = "http://127.0.0.1:8080"
		}
	}
	if devBase == "" {
		devBase = "http://127.0.0.1:3000"
	}
	base = strings.TrimRight(base, "/")
	devBase = strings.TrimRight(devBase, "/")
	return &testCtx{
		attachMode:     attach,
		requireBackend: true,
		keepViewport:   keepViewport,
		baseURL:        base,
		devBaseURL:     devBase,
		clickGap:       time.Second / time.Duration(cps),
	}
}

const (
	mobileViewportWidth  = 390
	mobileViewportHeight = 844
	mobileScaleFactor    = 2
)

func (t *testCtx) ensureSharedServer() error {
	if t.sharedServer != nil {
		return nil
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	_ = browser.CleanupPort(8080)
	// Launch the robot mock server
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "robot", "start", "--mock", "--local-only", "--web-port", "8080", "--hostname", "robot-test-client")
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
	t.sharedServer = cmd
	return nil
}

func (t *testCtx) ensureSharedBrowser(requireBackend bool) (*test_v2.BrowserSession, error) {
	if requireBackend {
		if err := t.ensureSharedServer(); err != nil {
			return nil, err
		}
	}
	if t.sharedBrowser != nil {
		return t.sharedBrowser, nil
	}
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

	// Append ?test=true to disable PWA auto-reload
	startURL := t.appURL("/?test=true")
	if !requireBackend {
		startURL = t.devURL("/?test=true")
	}

	if t.attachMode {
		// First check for a direct tunnel on 9222 (useful for Mac -> SSH -> WSL)
		if hasReachableDevtoolsWebSocket(9222) {
			session, err = test_v2.ConnectToBrowser(9222, "robot-dev")
		} else {
			session, err = start(false, "robot-dev", true, startURL)
		}
		t.activeAttachSession = true
	} else {
		session, err = start(true, "test", false, startURL)
		t.activeAttachSession = false
	}
	if err != nil {
		return nil, err
	}
	t.sharedBrowser = session

	tasks := chromedp.Tasks{
		chromedp.Evaluate(`window.sessionStorage.setItem('robot_test_mode', '1')`, nil),
		chromedp.Evaluate(fmt.Sprintf(`window.sessionStorage.setItem('robot_test_attach', %q)`, map[bool]string{true: "1", false: "0"}[t.activeAttachSession]), nil),
	}
	if !t.keepViewport {
		tasks = append(chromedp.Tasks{
			chromedp.EmulateViewport(mobileViewportWidth, mobileViewportHeight, chromedp.EmulateScale(mobileScaleFactor)),
			emulation.SetDeviceMetricsOverride(mobileViewportWidth, mobileViewportHeight, mobileScaleFactor, true),
			emulation.SetTouchEmulationEnabled(true),
		}, tasks...)
	}
	if err := t.sharedBrowser.Run(tasks); err != nil {
		return nil, err
	}
	return t.sharedBrowser, nil
}

func (t *testCtx) teardown() {
	if t.sharedBrowser != nil {
		if !t.activeAttachSession {
			t.sharedBrowser.Close()
		}
		t.sharedBrowser = nil
	}
	if t.sharedServer != nil {
		_ = t.sharedServer.Process.Kill()
		_, _ = t.sharedServer.Process.Wait()
		t.sharedServer = nil
	}
}

func (t *testCtx) browser() (*test_v2.BrowserSession, error) {
	return t.ensureSharedBrowser(t.requireBackend)
}

func (t *testCtx) logWait(label, detail string) {
	fmt.Printf("[WAIT] label=%s detail=%s\n", label, detail)
}

func (t *testCtx) logClick(kind, target, detail string) {
	fmt.Printf("[CLICK] kind=%s target=%s detail=%s\n", kind, target, detail)
}

func (t *testCtx) waitClickGap() {
	if t.clickGap <= 0 {
		t.lastClickAt = time.Now()
		return
	}
	if t.lastClickAt.IsZero() {
		t.lastClickAt = time.Now()
		return
	}
	nextAllowed := t.lastClickAt.Add(t.clickGap)
	now := time.Now()
	if now.Before(nextAllowed) {
		time.Sleep(nextAllowed.Sub(now))
	}
	t.lastClickAt = time.Now()
}

func (t *testCtx) waitAria(label, detail string) error {
	b, err := t.browser()
	if err != nil {
		return err
	}
	t.logWait(label, detail)
	return b.Run(test_v2.WaitForAriaLabel(label))
}

func (t *testCtx) waitAriaAttrEquals(label, attr, expected, detail string, timeout time.Duration) error {
	b, err := t.browser()
	if err != nil {
		return err
	}
	t.logWait(label, detail)
	return b.Run(test_v2.WaitForAriaLabelAttrEquals(label, attr, expected, timeout))
}

func (t *testCtx) clickAria(label, detail string) error {
	b, err := t.browser()
	if err != nil {
		return err
	}
	t.waitClickGap()
	t.logClick("aria", label, detail)
	return b.Run(test_v2.ClickAriaLabel(label))
}

func (t *testCtx) typeAndSubmitAria(label, text, detail string) error {
	b, err := t.browser()
	if err != nil {
		return err
	}
	t.waitClickGap()
	t.logClick("type", label, fmt.Sprintf("%s; text=%q", detail, text))
	return b.Run(test_v2.TypeAndSubmitAriaLabel(label, text))
}

func (t *testCtx) navigate(url string) error {
	b, err := t.browser()
	if err != nil {
		return err
	}
	return b.Run(chromedp.Navigate(url))
}

func (t *testCtx) appURL(path string) string {
	if path == "" {
		return t.baseURL
	}
	return t.baseURL + path
}

func (t *testCtx) devURL(path string) string {
	if path == "" {
		return t.devBaseURL
	}
	return t.devBaseURL + path
}

func (t *testCtx) captureShot(file string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	shot := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v1", "test", "screenshots", file)
	b, err := t.browser()
	if err != nil {
		return err
	}
	return b.CaptureScreenshot(shot)
}

func hasReachableDevtoolsWebSocket(port int) bool {
	client := &http.Client{Timeout: 700 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
