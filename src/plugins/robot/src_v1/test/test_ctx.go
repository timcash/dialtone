package test

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	chrome_app "dialtone/dev/plugins/chrome/src_v1/go"
	test_v2 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

func ariaSelector(label string) string {
	return fmt.Sprintf(`[aria-label="%s"]`, strings.ReplaceAll(label, `"`, `\"`))
}

type testCtx struct {
	repoRoot            string
	sharedServer        *exec.Cmd
	stepCtx             *test_v2.StepContext
	attachMode          bool
	activeAttachSession bool
	requireBackend      bool
	keepViewport        bool
	baseURL             string
	devBaseURL          string
	webPort             int
	natsPort            int
	natsWSPort          int
	browserInitialized  bool
	clickGap            time.Duration
	lastClickAt         time.Time
}

func newTestCtx() *testCtx {
	repoRoot, _ := findRepoRoot()
	fmt.Printf("[DEBUG] newTestCtx: repoRoot=%q\n", repoRoot)
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
	webPort, err := test_v2.PickFreePort()
	if err != nil {
		webPort = 8080
	}
	if base == "" {
		if attach {
			base = "http://127.0.0.1:3000"
		} else {
			base = fmt.Sprintf("http://127.0.0.1:%d", webPort)
		}
	}
	if devBase == "" {
		devBase = "http://127.0.0.1:3000"
	}
	natsPort, err := test_v2.PickFreePort()
	if err != nil {
		natsPort = 4222
	}
	natsWSPort, err := test_v2.PickFreePort()
	if err != nil {
		natsWSPort = 4223
	}
	base = strings.TrimRight(base, "/")
	devBase = strings.TrimRight(devBase, "/")
	return &testCtx{
		repoRoot:       repoRoot,
		attachMode:     attach,
		requireBackend: true,
		keepViewport:   keepViewport,
		baseURL:        base,
		devBaseURL:     devBase,
		webPort:        webPort,
		natsPort:       natsPort,
		natsWSPort:     natsWSPort,
		clickGap:       time.Second / time.Duration(cps),
	}
}

const (
	mobileViewportWidth  = 390
	mobileViewportHeight = 844
	mobileScaleFactor    = 1
)

func (t *testCtx) ensureSharedServer() error {
	if t.sharedServer != nil {
		return nil
	}

	_ = chrome_app.CleanupPort(t.webPort)
	// Launch the robot mock server
	cmd := exec.Command(filepath.Join(t.repoRoot, "dialtone.sh"), "robot", "src_v1", "serve")
	cmd.Dir = t.repoRoot
	cmd.Env = append(
		os.Environ(),
		"ROBOT_TSNET=0",
		fmt.Sprintf("ROBOT_WEB_PORT=%d", t.webPort),
		fmt.Sprintf("NATS_PORT=%d", t.natsPort),
		fmt.Sprintf("NATS_WS_PORT=%d", t.natsWSPort),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := t.waitHTTPReady(t.backendURL("/health"), 12*time.Second); err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return err
	}
	t.sharedServer = cmd
	return nil
}

func (t *testCtx) ensureSharedBrowser(requireBackend bool) (*test_v2.BrowserSession, error) {
	if t.stepCtx == nil {
		return nil, fmt.Errorf("step context not bound")
	}
	if requireBackend {
		if err := t.ensureSharedServer(); err != nil {
			return nil, err
		}
	}

	// Append ?test=true to disable PWA auto-reload
	startURL := t.appURL("/?test=true")
	if !requireBackend {
		startURL = t.devURL("/?test=true")
	}
	urlArg := ""
	if !t.browserInitialized {
		urlArg = startURL
	}

	if t.attachMode {
		_, err := t.stepCtx.EnsureBrowser(test_v2.BrowserOptions{
			Role:          "robot-dev",
			ReuseExisting: true,
			URL:           urlArg,
		})
		if err != nil {
			return nil, err
		}
		t.activeAttachSession = true
	} else {
		_, err := t.stepCtx.EnsureBrowser(test_v2.BrowserOptions{
			Headless:      true,
			Role:          "test",
			ReuseExisting: false,
			URL:           urlArg,
		})
		if err != nil {
			return nil, err
		}
		t.activeAttachSession = false
	}
	t.browserInitialized = true

	tasks := chromedp.Tasks{
		chromedp.Evaluate(`window.sessionStorage.setItem('robot_test_mode', '1')`, nil),
		chromedp.Evaluate(fmt.Sprintf(`window.sessionStorage.setItem('robot_test_attach', %q)`, map[bool]string{true: "1", false: "0"}[t.activeAttachSession]), nil),
	}
	if !t.keepViewport {
		tasks = append(
			chromedp.Tasks{
				chromedp.EmulateViewport(mobileViewportWidth, mobileViewportHeight, chromedp.EmulateScale(mobileScaleFactor)),
				emulation.SetDeviceMetricsOverride(mobileViewportWidth, mobileViewportHeight, mobileScaleFactor, true),
				emulation.SetTouchEmulationEnabled(true),
			},
			tasks...,
		)
	}
	if err := t.stepCtx.RunBrowser(tasks...); err != nil {
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
	t.logWait(label, detail)
	if t.stepCtx == nil {
		return fmt.Errorf("step context not bound")
	}
	if err := t.stepCtx.WaitForAriaLabel(label, 5*time.Second); err != nil {
		var href string
		var ready string
		var bodyLen int
		_ = t.stepCtx.RunBrowser(
			chromedp.Evaluate(`window.location.href`, &href),
			chromedp.Evaluate(`document.readyState`, &ready),
			chromedp.Evaluate(`(document.body && document.body.innerHTML ? document.body.innerHTML.length : 0)`, &bodyLen),
		)
		return fmt.Errorf("wait aria %q failed: %w (href=%q readyState=%q bodyLen=%d)", label, err, href, ready, bodyLen)
	}
	return nil
}

func (t *testCtx) waitAriaAttrEquals(label, attr, expected, detail string, timeout time.Duration) error {
	t.logWait(label, detail)
	if t.stepCtx == nil {
		return fmt.Errorf("step context not bound")
	}
	return t.stepCtx.WaitForAriaLabelAttrEquals(label, attr, expected, timeout)
}

func (t *testCtx) clickAria(label, detail string) error {
	if _, err := t.browser(); err != nil {
		return err
	}
	t.waitClickGap()
	t.logClick("aria", label, detail)
	if t.stepCtx == nil {
		return fmt.Errorf("step context not bound")
	}
	selector := ariaSelector(label)
	return t.stepCtx.WaitForBrowserMessageAfterAction(fmt.Sprintf("[TEST_ACTION] click aria=%s", label), 5*time.Second, func() error {
		if err := t.stepCtx.RunBrowserWithTimeout(
			3*time.Second,
			chromedp.ScrollIntoView(selector, chromedp.ByQuery),
			chromedp.Click(selector, chromedp.ByQuery),
		); err == nil {
			return nil
		}
		var center struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		}
		script := fmt.Sprintf(`(() => {
			const selector = %q;
			const el = document.querySelector(selector);
			if (!el) return null;
			const r = el.getBoundingClientRect();
			return { x: r.left + (r.width / 2), y: r.top + (r.height / 2) };
		})()`, selector)
		if err := t.stepCtx.RunBrowserWithTimeout(2*time.Second, chromedp.Evaluate(script, &center)); err != nil {
			return fmt.Errorf("click aria %q failed and fallback center lookup failed: %w", label, err)
		}
		if center.X <= 0 && center.Y <= 0 {
			return fmt.Errorf("click aria %q failed and fallback center is invalid", label)
		}
		if err := t.stepCtx.TapAt(center.X, center.Y); err != nil {
			return fmt.Errorf("click aria %q failed and tap fallback failed: %w", label, err)
		}
		return nil
	})
}

func (t *testCtx) typeAndSubmitAria(label, text, detail string) error {
	if _, err := t.browser(); err != nil {
		return err
	}
	t.waitClickGap()
	t.logClick("type", label, fmt.Sprintf("%s; text=%q", detail, text))
	if t.stepCtx == nil {
		return fmt.Errorf("step context not bound")
	}
	return t.stepCtx.WaitForBrowserMessageAfterAction(fmt.Sprintf("[TEST_ACTION] input aria=%s", label), 5*time.Second, func() error {
		return t.stepCtx.RunBrowserWithTimeout(5*time.Second, test_v2.TypeAndSubmitAriaLabel(label, text))
	})
}

func (t *testCtx) navigate(url string) error {
	if t.stepCtx == nil {
		return fmt.Errorf("step context not bound")
	}
	return t.stepCtx.RunBrowser(chromedp.Navigate(url))
}

func (t *testCtx) navigateSection(sectionID string) error {
	menuTargetBySection := map[string]string{
		"hero":     "Navigate Hero",
		"docs":     "Navigate Docs",
		"table":    "Navigate Telemetry",
		"three":    "Navigate Three",
		"xterm":    "Navigate Terminal",
		"video":    "Navigate Camera",
		"settings": "Navigate Settings",
	}
	sectionLabelByID := map[string]string{
		"hero":     "Hero Section",
		"docs":     "Docs Section",
		"table":    "Telemetry Section",
		"three":    "Three Section",
		"xterm":    "Xterm Section",
		"video":    "Video Section",
		"settings": "Settings Section",
	}
	target, ok := menuTargetBySection[sectionID]
	if !ok {
		return fmt.Errorf("unknown section id %q", sectionID)
	}
	if sectionID == "hero" {
		if err := t.navigate(t.appURL("/#hero")); err != nil {
			return err
		}
		return t.waitAria("Hero Section", "hero section visible")
	}
	if err := t.waitAria("Toggle Global Menu", "menu toggle visible"); err != nil {
		return err
	}
	if err := t.clickAria("Toggle Global Menu", "open menu"); err != nil {
		return err
	}
	if err := t.waitAria("Global Menu Panel", "menu visible"); err != nil {
		return err
	}
	if err := t.waitAria(target, "menu item visible"); err != nil {
		return err
	}
	if err := t.clickAria(target, "navigate section"); err != nil {
		return err
	}
	if label, ok := sectionLabelByID[sectionID]; ok {
		if err := t.waitAria(label, "section visible after navigation"); err != nil {
			return err
		}
	}
	return nil
}

func (t *testCtx) appURL(path string) string {
	if path == "" {
		return t.baseURL
	}
	return t.baseURL + path
}

func (t *testCtx) backendURL(path string) string {
	base := fmt.Sprintf("http://127.0.0.1:%d", t.webPort)
	if path == "" {
		return base
	}
	return base + path
}

func (t *testCtx) devURL(path string) string {
	if path == "" {
		return t.devBaseURL
	}
	return t.devBaseURL + path
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

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		path := filepath.Join(cwd, "dialtone.sh")
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("[DEBUG] findRepoRoot found %s at %s\n", path, cwd)
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("repo root not found")
		}
		cwd = parent
	}
}

func (t *testCtx) captureShot(file string) error {
	shot := filepath.Join(t.repoRoot, "src", "plugins", "robot", "src_v1", "test", "screenshots", file)
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
