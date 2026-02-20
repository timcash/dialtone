package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dialtone/dev/browser"
	test_v2 "dialtone/dev/plugins/test"
	chrome_app "dialtone/dev/plugins/chrome/app"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

type storyState struct {
	AProgramID string
	AAgentID   string
	LinkID     string
	BAgentID   string
	BProgramID string
	ProtoTxID  string
	ProtoRxID  string
}

type dagState struct {
	LastCreatedNodeID string `json:"lastCreatedNodeId"`
	ActiveLayerID     string `json:"activeLayerId"`
	SelectedNodeID    string `json:"selectedNodeId"`
	HistoryDepth      int    `json:"historyDepth"`
}

type projectedPoint struct {
	OK bool `json:"ok"`
	X  int  `json:"x"`
	Y  int  `json:"y"`
}

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
	maxClicksPerStep    int
	stepCtx             *test_v2.StepContext
	stepClicks          int
	story               storyState
	lastClickAt         time.Time
}

func newTestCtx() *testCtx {
	attach := os.Getenv("DAG_TEST_ATTACH") == "1"
	keepViewport := strings.TrimSpace(os.Getenv("DAG_TEST_KEEP_VIEWPORT")) == "1"
	if attach && !keepViewport {
		// In attach mode, preserve the user's current browser viewport unless explicitly overridden.
		keepViewport = true
	}
	base := strings.TrimSpace(os.Getenv("DAG_TEST_BASE_URL"))
	devBase := strings.TrimSpace(os.Getenv("DAG_TEST_DEV_BASE_URL"))
	cpsRaw := strings.TrimSpace(os.Getenv("DAG_TEST_CPS"))
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
		attachMode:       attach,
		requireBackend:   true,
		keepViewport:     keepViewport,
		baseURL:          base,
		devBaseURL:       devBase,
		clickGap:         time.Second / time.Duration(cps),
		maxClicksPerStep: 4,
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
	if t.attachMode {
		if !hasAttachableDagDevBrowser() {
			return nil, fmt.Errorf("DAG_TEST_ATTACH=1 requires a running Dialtone debug browser session (role=dag-dev); regular Chrome windows cannot be attached")
		}
		session, err = start(false, "dag-dev", true, t.devURL("/#dag-3d-stage"))
		t.activeAttachSession = true
	} else {
		startURL := t.appURL("/#dag-3d-stage")
		if !requireBackend {
			startURL = t.devURL("/#dag-3d-stage")
		}
		session, err = start(true, "test", false, startURL)
		t.activeAttachSession = false
	}
	if err != nil {
		return nil, err
	}
	t.sharedBrowser = session
	tasks := chromedp.Tasks{
		chromedp.Evaluate(`window.sessionStorage.setItem('dag_test_mode', '1')`, nil),
		chromedp.Evaluate(fmt.Sprintf(`window.sessionStorage.setItem('dag_test_attach', %q)`, map[bool]string{true: "1", false: "0"}[t.activeAttachSession]), nil),
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

func hasAttachableDagDevBrowser() bool {
	procs, err := chrome_app.ListResources(true)
	if err != nil {
		return false
	}
	for _, p := range procs {
		if p.Origin != "Dialtone" || p.Role != "dag-dev" || p.IsHeadless {
			continue
		}
		if p.DebugPort > 0 && hasReachableDevtoolsWebSocket(p.DebugPort) {
			return true
		}
	}
	return false
}

func hasReachableDevtoolsWebSocket(port int) bool {
	client := &http.Client{Timeout: 700 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false
	}
	var meta struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		return false
	}
	return strings.HasPrefix(meta.WebSocketDebuggerURL, "ws://")
}

func (t *testCtx) browser() (*test_v2.BrowserSession, error) {
	return t.ensureSharedBrowser(t.requireBackend)
}

func (t *testCtx) setRequireBackend(required bool) {
	t.requireBackend = required
}

func (t *testCtx) runEval(js string, out any) error {
	b, err := t.browser()
	if err != nil {
		return err
	}
	return b.Run(chromedp.Evaluate(js, out))
}

func (t *testCtx) appendThought(text string) {
	_ = t.runEval(fmt.Sprintf(`(() => {
		const api = window.dagHitTestDebug;
		if (api && typeof api.appendThought === 'function') api.appendThought(%q);
		return true;
	})()`, text), nil)
}

func toSlug(label string) string {
	v := strings.ToLower(strings.TrimSpace(label))
	repl := strings.NewReplacer(" ", "-", "|", "-", "_", "-", ":", "", "'", "", "\"", "", "(", "", ")", "")
	return repl.Replace(v)
}

func (t *testCtx) beginStep(sc *test_v2.StepContext) {
	t.stepCtx = sc
	t.stepClicks = 0
	t.lastClickAt = time.Time{}
	t.logf("STEP> begin %s", sc.Name)
}

func (t *testCtx) logf(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

func (t *testCtx) userf(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	t.logf("%s", line)
	t.appendThought(line)
}

func (t *testCtx) logWait(label, detail string) {
	t.logf("WAIT> aria-%s (%s)", toSlug(label), detail)
}

func (t *testCtx) logClick(kind, target, detail string) {
	switch kind {
	case "aria":
		t.userf("USER> click aria-%s", toSlug(target))
	case "node":
		t.userf("USER> click node-%s", toSlug(target))
	case "canvas":
		t.userf("USER> click canvas-%s", toSlug(target))
	default:
		t.userf("USER> click %s-%s", toSlug(kind), toSlug(target))
	}
	t.logf("CLICK> kind=%s target=%s detail=%s", kind, target, detail)
}

func (t *testCtx) markClick() error {
	t.stepClicks += 1
	if t.stepCtx != nil && t.stepClicks > t.maxClicksPerStep {
		return fmt.Errorf("step %q exceeded max clicks per step (%d)", t.stepCtx.Name, t.maxClicksPerStep)
	}
	return nil
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
	if err := t.markClick(); err != nil {
		return err
	}
	t.waitClickGap()
	t.logClick("aria", label, detail)
	return b.Run(test_v2.ClickAriaLabel(label))
}

func (t *testCtx) typeAria(label, value, detail string) error {
	b, err := t.browser()
	if err != nil {
		return err
	}
	t.userf("USER> type aria-%s %q", toSlug(label), value)
	t.logf("TYPE> aria-%s (%s)", toSlug(label), detail)
	return b.Run(test_v2.TypeAriaLabel(label, value))
}

func (t *testCtx) pressEnterAria(label, detail string) error {
	b, err := t.browser()
	if err != nil {
		return err
	}
	t.userf("USER> press-enter aria-%s", toSlug(label))
	t.logf("KEY> enter aria-%s (%s)", toSlug(label), detail)
	return b.Run(test_v2.PressEnterAriaLabel(label))
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

func (t *testCtx) ensureBackendStopped() {
	if t.sharedServer != nil {
		_ = t.sharedServer.Process.Kill()
		_, _ = t.sharedServer.Process.Wait()
		t.sharedServer = nil
	}
	_ = browser.CleanupPort(8080)
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

func (t *testCtx) waitLogTerminalContains(substr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var ok bool
		err := t.runEval(fmt.Sprintf(`(() => {
			const root = document.querySelector("[aria-label='Log Terminal']");
			if (!root) return false;
			const text = String(root.textContent || '');
			return text.includes(%q);
		})()`, substr), &ok)
		if err == nil && ok {
			t.logf("ASSERT> log-terminal contains %q", substr)
			return nil
		}
		time.Sleep(120 * time.Millisecond)
	}
	return fmt.Errorf("log terminal did not contain %q within %s", substr, timeout)
}

func (t *testCtx) captureShot(file string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	shot := filepath.Join(repoRoot, "src", "plugins", "dag", "src_v3", "screenshots", file)
	b, err := t.browser()
	if err != nil {
		return err
	}
	return b.CaptureScreenshot(shot)
}

func (t *testCtx) getMode() (string, error) {
	var mode string
	err := t.runEval(`(() => {
		const el = document.querySelector("[aria-label='DAG Mode']");
		return el ? String(el.getAttribute('data-mode') || '') : '';
	})()`, &mode)
	return mode, err
}

func (t *testCtx) ensureMode(mode string) error {
	for i := 0; i < 8; i++ {
		current, err := t.getMode()
		if err != nil {
			return err
		}
		if current == mode {
			return nil
		}
		t.logClick("mode", "DAG Mode", "target="+mode)
		if err := t.clickAria("DAG Mode", "switch mode"); err != nil {
			return err
		}
	}
	return fmt.Errorf("could not switch mode to %q", mode)
}

func (t *testCtx) assertMode(expected string) error {
	mode, err := t.getMode()
	if err != nil {
		return err
	}
	if mode != expected {
		return fmt.Errorf("thumb mode mismatch: got=%q expected=%q", mode, expected)
	}
	t.logf("ASSERT> mode=%s", mode)
	return nil
}

func (t *testCtx) clickAction(mode, actionID string) error {
	b, err := t.browser()
	if err != nil {
		return err
	}
	if mode != "" {
		if err := t.ensureMode(mode); err != nil {
			return err
		}
	}
	label, err := t.getActionLabel(actionID)
	if err != nil {
		return err
	}
	if label == "" {
		return fmt.Errorf("could not resolve label for action %q in mode %q", actionID, mode)
	}
	detail := "mode=" + mode + ";action=" + actionID + ";label=" + label
	if err := t.waitAria(label, "action button must exist before click"); err != nil {
		return err
	}
	if err := t.markClick(); err != nil {
		return err
	}
	t.waitClickGap()
	t.logClick("aria", label, detail)
	return b.Run(test_v2.ClickAriaLabel(label))
}

func (t *testCtx) getActionLabel(actionID string) (string, error) {
	var label string
	err := t.runEval(fmt.Sprintf(`(() => {
		const root = document.querySelector("[aria-label='DAG Mode Form']");
		if (!root) return "";
		const el = root.querySelector("button[data-action='%s']");
		if (!el) return "";
		const aria = String(el.getAttribute('aria-label') || '').trim();
		const text = String(el.textContent || '').trim();
		return aria || text;
	})()`, actionID), &label)
	return strings.TrimSpace(label), err
}

func (t *testCtx) clickCanvas(x, y int, detail string) error {
	if err := t.markClick(); err != nil {
		return err
	}
	t.waitClickGap()
	t.logClick("canvas", "Three Canvas", fmt.Sprintf("%s;x=%d,y=%d", detail, x, y))
	return t.runEval(fmt.Sprintf(`(() => {
		const canvas = document.querySelector("[aria-label='Three Canvas']");
		if (!canvas) return false;
		canvas.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, clientX: %d, clientY: %d, view: window }));
		return true;
	})()`, x, y), nil)
}

func (t *testCtx) getProjectedPoint(nodeID string) (projectedPoint, error) {
	var p projectedPoint
	err := t.runEval(fmt.Sprintf(`(() => {
		const api = window.dagHitTestDebug;
		if (!api || typeof api.getProjectedPoint !== 'function') return { ok: false, x: 0, y: 0 };
		return api.getProjectedPoint(%q);
	})()`, nodeID), &p)
	return p, err
}

func (t *testCtx) clickNode(nodeID string) error {
	p, err := t.getProjectedPoint(nodeID)
	if err != nil {
		return err
	}
	if !p.OK {
		return fmt.Errorf("projected point not found for node %s", nodeID)
	}
	if err := t.markClick(); err != nil {
		return err
	}
	t.waitClickGap()
	t.logClick("node", nodeID, fmt.Sprintf("x=%d,y=%d", p.X, p.Y))
	return t.runEval(fmt.Sprintf(`(() => {
		const canvas = document.querySelector("[aria-label='Three Canvas']");
		if (!canvas) return false;
		canvas.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, clientX: %d, clientY: %d, view: window }));
		return true;
	})()`, p.X, p.Y), nil)
}

func (t *testCtx) renameSelected(text string) error {
	b, err := t.browser()
	if err != nil {
		return err
	}
	if err := t.ensureMode("graph"); err != nil {
		return err
	}
	if err := b.Run(chromedp.SetValue("[aria-label='DAG Label Input']", text, chromedp.ByQuery)); err != nil {
		return err
	}
	t.logClick("rename_submit", "DAG Rename", text)
	return t.clickAria("DAG Rename", "submit rename")
}

func (t *testCtx) renameSelectedNoModeSwitch(text string) error {
	b, err := t.browser()
	if err != nil {
		return err
	}
	if err := b.Run(chromedp.SetValue("[aria-label='DAG Label Input']", text, chromedp.ByQuery)); err != nil {
		return err
	}
	t.logClick("rename_submit", "DAG Rename", text)
	return t.clickAria("DAG Rename", "submit rename")
}

func (t *testCtx) setRenameInput(text string) error {
	b, err := t.browser()
	if err != nil {
		return err
	}
	t.logf("INPUT> dag-label-input=%q", text)
	return b.Run(chromedp.SetValue("[aria-label='DAG Label Input']", text, chromedp.ByQuery))
}

func (t *testCtx) lastCreatedNodeID() (string, error) {
	var st dagState
	err := t.runEval(`(() => {
		const api = window.dagHitTestDebug;
		if (!api || typeof api.getState !== 'function') return { lastCreatedNodeId: '' };
		return api.getState();
	})()`, &st)
	return strings.TrimSpace(st.LastCreatedNodeID), err
}

func (t *testCtx) state() (dagState, error) {
	var st dagState
	err := t.runEval(`(() => {
		const api = window.dagHitTestDebug;
		if (!api || typeof api.getState !== 'function') return { lastCreatedNodeId: '', activeLayerId: '', selectedNodeId: '', historyDepth: 0 };
		return api.getState();
	})()`, &st)
	return st, err
}

type cameraNodeMetric struct {
	OK       bool    `json:"ok"`
	Distance float64 `json:"distance"`
	CamY     float64 `json:"camY"`
	NodeX    float64 `json:"nodeX"`
	NodeY    float64 `json:"nodeY"`
	NodeZ    float64 `json:"nodeZ"`
}

func (t *testCtx) cameraDistanceToNode(nodeID string) (cameraNodeMetric, error) {
	var out cameraNodeMetric
	err := t.runEval(fmt.Sprintf(`(() => {
		const api = window.dagHitTestDebug;
		if (!api || typeof api.getCameraTransform !== 'function' || typeof api.getNodeWorldPosition !== 'function') return { ok:false, distance:0, camY:0, nodeX:0, nodeY:0, nodeZ:0 };
		const cam = api.getCameraTransform();
		const node = api.getNodeWorldPosition(%q);
		if (!node || !node.ok) return { ok:false, distance:0, camY:0, nodeX:0, nodeY:0, nodeZ:0 };
		const dx = cam.position.x - node.x;
		const dy = cam.position.y - node.y;
		const dz = cam.position.z - node.z;
		return { ok:true, distance: Math.sqrt(dx*dx+dy*dy+dz*dz), camY: cam.position.y, nodeX:node.x, nodeY:node.y, nodeZ:node.z };
	})()`, nodeID), &out)
	return out, err
}

func (t *testCtx) assertProjectedInCanvas(nodeID string) error {
	var dims struct {
		W int `json:"w"`
		H int `json:"h"`
	}
	if err := t.runEval(`(() => {
		const c = document.querySelector("[aria-label='Three Canvas']");
		if (!c) return { w: 0, h: 0 };
		const r = c.getBoundingClientRect();
		return { w: Math.round(r.width), h: Math.round(r.height) };
	})()`, &dims); err != nil {
		return err
	}
	p, err := t.getProjectedPoint(nodeID)
	if err != nil {
		return err
	}
	if !p.OK {
		return fmt.Errorf("node %s projection missing", nodeID)
	}
	if p.X < 0 || p.Y < 0 || p.X > dims.W || p.Y > dims.H {
		return fmt.Errorf("node %s projected outside viewport (%d,%d) not in [0..%d,0..%d]", nodeID, p.X, p.Y, dims.W, dims.H)
	}
	t.logf("ASSERT> node=%s projected=(%d,%d) canvas=(%d,%d)", nodeID, p.X, p.Y, dims.W, dims.H)
	return nil
}

func (t *testCtx) assertNodeCameraDistance(nodeID string, expected, tolerance float64) error {
	m, err := t.cameraDistanceToNode(nodeID)
	if err != nil {
		return err
	}
	if !m.OK {
		return fmt.Errorf("camera/node metric not available for %s", nodeID)
	}
	if math.Abs(m.Distance-expected) > tolerance {
		return fmt.Errorf("camera distance for %s out of range: got %.2f expected %.2f±%.2f", nodeID, m.Distance, expected, tolerance)
	}
	t.logf("ASSERT> camera_distance node=%s got=%.2f expected=%.2f±%.2f", nodeID, m.Distance, expected, tolerance)
	return nil
}

func (t *testCtx) assertActiveLayer(expected string) error {
	st, err := t.state()
	if err != nil {
		return err
	}
	if st.ActiveLayerID != expected {
		return fmt.Errorf("active layer mismatch: got=%q expected=%q", st.ActiveLayerID, expected)
	}
	t.logf("ASSERT> active_layer=%s", st.ActiveLayerID)
	return nil
}

func (t *testCtx) assertCameraAboveNode(nodeID string, minDeltaY float64) error {
	m, err := t.cameraDistanceToNode(nodeID)
	if err != nil {
		return err
	}
	if !m.OK {
		return fmt.Errorf("camera/node metric not available for %s", nodeID)
	}
	if (m.CamY - m.NodeY) < minDeltaY {
		return fmt.Errorf("camera Y not above node enough for %s: camY=%.2f nodeY=%.2f minDelta=%.2f", nodeID, m.CamY, m.NodeY, minDeltaY)
	}
	t.logf("ASSERT> camera_above node=%s camY=%.2f nodeY=%.2f delta=%.2f", nodeID, m.CamY, m.NodeY, m.CamY-m.NodeY)
	return nil
}

func (t *testCtx) assertHistoryDepthAtLeast(min int) error {
	st, err := t.state()
	if err != nil {
		return err
	}
	if st.HistoryDepth < min {
		return fmt.Errorf("history depth too small: got=%d expected>=%d", st.HistoryDepth, min)
	}
	t.logf("ASSERT> history_depth=%d", st.HistoryDepth)
	return nil
}
