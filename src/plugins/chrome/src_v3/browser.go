package src_v3

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	cdbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	cdpage "github.com/chromedp/cdproto/page"
	cdruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

const managedConsoleHookScript = `(() => {
	if (window.__dialtoneConsoleHookInstalled) {
		return true;
	}
	const normalize = (value) => {
		if (typeof value === 'string') {
			return value;
		}
		try {
			return JSON.stringify(value);
		} catch (_) {}
		try {
			return String(value);
		} catch (_) {
			return '[unprintable]';
		}
	};
	const lines = Array.isArray(window.__dialtoneConsoleLines) ? window.__dialtoneConsoleLines : [];
	window.__dialtoneConsoleLines = lines;
	const capture = (...args) => {
		const line = args
			.map(normalize)
			.filter((part) => typeof part === 'string' && part.trim() !== '')
			.join(' ')
			.trim();
		if (!line) {
			return;
		}
		lines.push(line);
		if (lines.length > 200) {
			lines.splice(0, lines.length - 200);
		}
	};
	for (const level of ['log', 'info', 'warn', 'error', 'debug']) {
		const original = typeof console[level] === 'function' ? console[level].bind(console) : () => {};
		console[level] = (...args) => {
			try {
				capture(...args);
			} catch (_) {}
			return original(...args);
		};
	}
	window.__dialtoneConsoleHookInstalled = true;
	return true;
})()`

const managedConsoleReadScript = `(() => {
	if (!Array.isArray(window.__dialtoneConsoleLines)) {
		return [];
	}
	return window.__dialtoneConsoleLines.slice(-200);
})()`

const (
	gracefulBrowserCloseCommandTimeout = 2 * time.Second
	gracefulBrowserExitWaitTimeout     = 4 * time.Second
)

var (
	gracefulCloseBrowserFunc  = gracefulCloseBrowser
	killBrowserPIDFunc        = killPID
	waitForBrowserPIDExitFunc = waitForPIDExit
	cleanupProfileLocksFunc   = cleanupChromeProfileLocks
	persistDaemonStateFunc    = func(d *daemonState) { d.persistState() }
)

func (d *daemonState) isBrowserAlive(pid int, port int) bool {
	if pid <= 0 {
		return false
	}
	client := &http.Client{Timeout: 1000 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		if runtime.GOOS == "windows" {
			if localListenPortOpen(port) {
				return true
			}
			if processAlive(pid) {
				return true
			}
			if pids, perr := chromeBrowserPIDsForRole(d.role, d.profileDir, port); perr == nil {
				for _, candidate := range pids {
					if candidate == pid {
						return true
					}
				}
				if len(pids) > 0 {
					return true
				}
			}
			return false
		}
		proc, findErr := os.FindProcess(pid)
		if findErr != nil || proc == nil {
			return false
		}
		return proc.Signal(syscall.Signal(0)) == nil
	}
	resp.Body.Close()
	if runtime.GOOS == "windows" {
		return true
	}
	proc, err := os.FindProcess(pid)
	if err != nil || proc == nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func (d *daemonState) ensureBrowser() error {
	d.mu.Lock()
	pid := d.browserPID
	port := d.chromePort
	hasAlloc := d.allocCtx != nil
	hasTab := d.tabCtx != nil
	unexpected := d.unexpectedErr
	d.mu.Unlock()

	if pid > 0 && d.isBrowserAlive(pid, port) {
		if runtime.GOOS == "windows" {
			if err := ensureSingleChromeProcessForRole(d.role, d.profileDir, d.chromePort, pid); err != nil {
				logs.Error("chrome src_v3 failed to prune duplicate browsers: %v", err)
			}
		}
		if !hasAlloc || unexpected != nil {
			if err := d.attachToRunningBrowser(); err == nil {
				if unexpected != nil {
					d.clearManagedTab()
					return d.ensureManagedTab()
				}
				if !hasTab {
					return d.ensureManagedTab()
				}
				return nil
			}
		}
		if !hasTab {
			return d.ensureManagedTab()
		}
		return nil
	}

	d.mu.Lock()
	if d.browserPID > 0 && d.allocCtx != nil {
		pid := d.browserPID
		port := d.chromePort
		hasTab := d.tabCtx != nil
		d.mu.Unlock()
		if d.isBrowserAlive(pid, port) {
			if !hasTab {
				return d.ensureManagedTab()
			}
			return nil
		}
		logs.Error("chrome src_v3 detected browser death, clearing stale session and restarting")
		d.mu.Lock()
		cancelAlloc := d.cancelAlloc
		cancelTab := d.cancelTab
		d.allocCtx = nil
		d.cancelAlloc = nil
		d.tabCtx = nil
		d.cancelTab = nil
		d.browserPID = 0
		d.browserWS = ""
		d.currentURL = ""
		d.managedTarget = ""
		d.consoleLines = nil
		d.unexpectedErr = nil
		d.intentionalStop = true
		d.mu.Unlock()
		if cancelTab != nil {
			cancelTab()
		}
		if cancelAlloc != nil {
			cancelAlloc()
		}
		d.mu.Lock()
		d.intentionalStop = false
		d.mu.Unlock()
		d.persistState()
	} else {
		d.mu.Unlock()
	}

	if runtime.GOOS == "windows" {
		if err := cleanupChromeProfileLocks(d.profileDir); err != nil {
			logs.Error("chrome src_v3 failed to clean profile locks: %v", err)
		}
		if err := ensureSingleChromeProcessForRole(d.role, d.profileDir, d.chromePort, 0); err != nil {
			logs.Error("chrome src_v3 failed to clear duplicate browsers before start: %v", err)
		}
	}

	logs.Info("chrome src_v3 starting browser: %s %v", d.chromePath, d.browserArgs())
	pid, err := d.startBrowserProcess()
	if err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	d.mu.Lock()
	d.browserPID = pid
	d.intentionalStop = false
	d.unexpectedErr = nil
	d.mu.Unlock()

	wsURL, err := waitForWebSocket(d.chromePort, 25*time.Second)
	if err != nil {
		_ = killPID(pid)
		return err
	}
	if runtime.GOOS == "windows" {
		actualPID, err := detectBrowserPID(d.chromePort, d.role, d.profileDir)
		if err == nil && actualPID > 0 {
			logs.Info("chrome src_v3 refined browser PID: %d", actualPID)
			d.mu.Lock()
			d.browserPID = actualPID
			d.mu.Unlock()
			if err := ensureSingleChromeProcessForRole(d.role, d.profileDir, d.chromePort, actualPID); err != nil {
				logs.Error("chrome src_v3 failed to prune duplicate browsers after start: %v", err)
			}
		} else {
			_ = killPID(pid)
			return fmt.Errorf("failed to detect real chrome browser process on port %d", d.chromePort)
		}
	} else if actualPID, err := detectBrowserPID(d.chromePort, d.role, d.profileDir); err == nil && actualPID == pid {
		logs.Info("chrome src_v3 refined browser PID: %d", actualPID)
		d.mu.Lock()
		d.browserPID = actualPID
		d.mu.Unlock()
	}

	d.installAllocator(wsURL)
	d.persistState()

	return d.ensureManagedTab()
}

func (d *daemonState) ensureManagedTab() error {
	d.mu.Lock()
	if d.tabCtx != nil {
		d.mu.Unlock()
		return nil
	}
	allocCtx := d.allocCtx
	chromePort := d.chromePort
	d.mu.Unlock()
	if allocCtx == nil {
		return errBrowserClosed
	}

	if targetID, err := firstPageTargetID(chromePort); err == nil && strings.TrimSpace(targetID) != "" {
		tabCtx, cancel, attachErr := newManagedTabContext(allocCtx, targetID)
		if attachErr == nil {
			d.attachManagedTab(tabCtx, cancel)
			return nil
		}
		logs.Warn("chrome src_v3 failed to bind first page target %s: %v", targetID, attachErr)
	}

	tabCtx, cancel, err := newManagedTabContext(allocCtx, "")
	if err != nil {
		return err
	}
	d.attachManagedTab(tabCtx, cancel)
	return nil
}

func (d *daemonState) clearManagedTab() {
	d.mu.Lock()
	cancelTab := d.cancelTab
	d.cancelTab = nil
	d.tabCtx = nil
	d.currentURL = "about:blank"
	d.managedTarget = ""
	d.mu.Unlock()
	if cancelTab != nil {
		cancelTab()
	}
}

func newManagedTabContext(allocCtx context.Context, targetID string) (context.Context, context.CancelFunc, error) {
	ctxOpts := []chromedp.ContextOption{}
	if strings.TrimSpace(targetID) != "" {
		ctxOpts = append(ctxOpts, chromedp.WithTargetID(target.ID(strings.TrimSpace(targetID))))
	}
	tabCtx, cancel := chromedp.NewContext(allocCtx, ctxOpts...)
	if err := primeManagedTabContext(tabCtx); err != nil {
		cancel()
		return nil, nil, err
	}
	return tabCtx, cancel, nil
}

func primeManagedTabContext(tabCtx context.Context) error {
	var readyState string
	return chromedp.Run(tabCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return cdruntime.Enable().Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return cdpage.Enable().Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := cdpage.AddScriptToEvaluateOnNewDocument(managedConsoleHookScript).Do(ctx)
			return err
		}),
		chromedp.Evaluate(managedConsoleHookScript, nil),
		chromedp.Evaluate(`document.readyState`, &readyState),
	)
}

type devtoolsTargetInfo struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

func firstPageTargetID(port int) (string, error) {
	if port <= 0 {
		return "", fmt.Errorf("chrome debug port unavailable")
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected devtools target status: %s", resp.Status)
	}
	var targets []devtoolsTargetInfo
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return "", err
	}
	for _, t := range targets {
		if strings.TrimSpace(t.Type) != "page" {
			continue
		}
		if id := strings.TrimSpace(t.ID); id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("no page target found")
}

func (d *daemonState) attachManagedTab(tabCtx context.Context, cancel context.CancelFunc) {
	managedTarget := ""
	if chromeCtx := chromedp.FromContext(tabCtx); chromeCtx != nil && chromeCtx.Target != nil {
		if targetID := strings.TrimSpace(string(chromeCtx.Target.TargetID)); targetID != "" {
			managedTarget = targetID
		}
	}
	d.mu.Lock()
	d.tabCtx = tabCtx
	d.cancelTab = cancel
	d.managedTarget = managedTarget
	d.currentURL = "about:blank"
	d.consoleLines = nil
	d.mu.Unlock()
	d.persistState()
	chromedp.ListenTarget(tabCtx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *cdruntime.EventConsoleAPICalled:
			parts := make([]string, 0, len(ev.Args))
			for _, arg := range ev.Args {
				parts = append(parts, consoleArgText(arg))
			}
			d.appendConsoleLine(strings.TrimSpace(strings.Join(parts, " ")))
		case *cdruntime.EventExceptionThrown:
			d.appendConsoleLine(strings.TrimSpace(ev.ExceptionDetails.Text))
		}
	})
	if strings.TrimSpace(managedTarget) == "" {
		logs.Warn("chrome src_v3 managed target unresolved for role=%s; skipping immediate tab prune", d.role)
		return
	}
	if err := d.pruneExtraPageTargets(); err != nil {
		logs.Warn("chrome src_v3 unable to prune extra tabs for role=%s: %v", d.role, err)
	}
}

func (d *daemonState) pruneExtraPageTargets() error {
	d.mu.Lock()
	tabCtx := d.tabCtx
	managedTarget := strings.TrimSpace(d.managedTarget)
	d.mu.Unlock()
	if tabCtx == nil {
		return errBrowserClosed
	}
	chromeCtx := chromedp.FromContext(tabCtx)
	if chromeCtx == nil || chromeCtx.Browser == nil {
		return fmt.Errorf("browser executor unavailable for tab pruning")
	}
	if managedTarget == "" && chromeCtx.Target != nil {
		managedTarget = strings.TrimSpace(string(chromeCtx.Target.TargetID))
	}
	if managedTarget == "" {
		return fmt.Errorf("managed target unavailable for tab pruning")
	}
	browserExecCtx := cdp.WithExecutor(tabCtx, chromeCtx.Browser)
	targets, err := target.GetTargets().Do(browserExecCtx)
	if err != nil {
		return err
	}
	pageTargets := make([]string, 0, len(targets))
	managedPresent := false
	for _, t := range targets {
		if t == nil || t.Type != "page" {
			continue
		}
		targetID := strings.TrimSpace(string(t.TargetID))
		if targetID == "" {
			continue
		}
		pageTargets = append(pageTargets, targetID)
		if targetID == managedTarget {
			managedPresent = true
		}
	}
	if managedTarget == "" && len(pageTargets) == 1 {
		managedTarget = pageTargets[0]
		managedPresent = true
		d.mu.Lock()
		d.managedTarget = managedTarget
		d.mu.Unlock()
	}
	if managedTarget != "" && !managedPresent {
		if len(pageTargets) == 1 {
			managedTarget = pageTargets[0]
			managedPresent = true
			d.mu.Lock()
			d.managedTarget = managedTarget
			d.mu.Unlock()
		} else {
			return fmt.Errorf("managed target %q not found among page targets", managedTarget)
		}
	}
	if !managedPresent && len(pageTargets) > 0 {
		return fmt.Errorf("managed target unavailable for %d page targets", len(pageTargets))
	}
	for _, t := range targets {
		if t == nil || t.Type != "page" {
			continue
		}
		targetID := strings.TrimSpace(string(t.TargetID))
		if targetID == "" || targetID == managedTarget {
			continue
		}
		if err := target.CloseTarget(t.TargetID).Do(browserExecCtx); err != nil {
			logs.Warn("chrome src_v3 failed closing extra target %s: %v", targetID, err)
		}
	}
	return nil
}

func (d *daemonState) recreateManagedTab() error {
	d.mu.Lock()
	cancelTab := d.cancelTab
	d.cancelTab = nil
	d.tabCtx = nil
	d.currentURL = "about:blank"
	d.managedTarget = ""
	d.mu.Unlock()
	if cancelTab != nil {
		cancelTab()
	}
	return d.ensureManagedTab()
}

func (d *daemonState) attachToRunningBrowser() error {
	wsURL, err := waitForWebSocket(d.chromePort, 3*time.Second)
	if err != nil {
		return err
	}
	d.installAllocator(wsURL)
	if actualPID, err := detectBrowserPID(d.chromePort, d.role, d.profileDir); err == nil && actualPID > 0 {
		d.mu.Lock()
		d.browserPID = actualPID
		d.mu.Unlock()
		if runtime.GOOS == "windows" {
			if err := ensureSingleChromeProcessForRole(d.role, d.profileDir, d.chromePort, actualPID); err != nil {
				logs.Error("chrome src_v3 failed to prune duplicate browsers after attach: %v", err)
			}
		}
	}
	d.persistState()
	return nil
}

func (d *daemonState) installAllocator(wsURL string) {
	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	d.mu.Lock()
	if d.cancelAlloc != nil {
		d.cancelAlloc()
	}
	d.allocCtx = allocCtx
	d.cancelAlloc = cancel
	d.browserWS = wsURL
	d.unexpectedErr = nil
	d.intentionalStop = false
	d.mu.Unlock()

	go func() {
		<-allocCtx.Done()
		d.mu.Lock()
		if !d.intentionalStop && d.unexpectedErr == nil {
			logs.Error("chrome src_v3 allocator connection closed")
			d.unexpectedErr = fmt.Errorf("browser allocator connection lost")
		}
		d.mu.Unlock()
		d.persistState()
	}()
}

func (d *daemonState) startBrowserProcess() (int, error) {
	if runtime.GOOS == "windows" {
		return startDetachedWindowsProcess(d.chromePath, d.browserArgs(), chromeHeadlessEnabled())
	}
	cmd := exec.Command(d.chromePath, d.browserArgs()...)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

func (d *daemonState) browserArgs() []string {
	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", d.chromePort),
		"--remote-debugging-address=127.0.0.1",
		"--remote-allow-origins=*",
		"--user-data-dir=" + d.profileDir,
		"--dialtone-role=" + d.role,
		"--dialtone-managed-profile=" + d.profileDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-gpu",
	}
	if runtime.GOOS == "windows" {
		if chromeHeadlessEnabled() {
			args = append(args,
				"--headless=new",
				"--hide-scrollbars",
			)
		}
		args = append(args, "--window-size=1440,960")
	}
	args = append(args, "about:blank")
	return args
}

func (d *daemonState) withManagedContext(timeout time.Duration, fn func(context.Context) error) error {
	if err := d.ensureManagedTab(); err != nil {
		return err
	}
	err := d.runManaged(timeout, fn)
	if !shouldRecreateManagedTab(err) {
		return err
	}
	if recreateErr := d.recreateManagedTab(); recreateErr != nil {
		return err
	}
	return d.runManaged(timeout, fn)
}

func (d *daemonState) runManaged(timeout time.Duration, fn func(context.Context) error) error {
	d.mu.Lock()
	if d.unexpectedErr != nil {
		err := d.unexpectedErr
		d.mu.Unlock()
		return err
	}
	if d.allocCtx == nil || d.tabCtx == nil {
		d.mu.Unlock()
		return errBrowserClosed
	}
	tabCtx := d.tabCtx
	d.mu.Unlock()
	runCtx, runCancel := context.WithTimeout(tabCtx, timeout)
	defer runCancel()
	return fn(runCtx)
}

func shouldRecreateManagedTab(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "context canceled") ||
		strings.Contains(msg, "invalid context") ||
		strings.Contains(msg, "session closed") ||
		strings.Contains(msg, "target closed")
}

func (d *daemonState) openNewTab(rawURL string) error {
	url := normalizeURL(rawURL)
	if url == "" {
		url = "about:blank"
	}
	d.mu.Lock()
	allocCtx := d.allocCtx
	if d.cancelTab != nil {
		d.cancelTab()
		d.cancelTab = nil
		d.tabCtx = nil
	}
	d.currentURL = ""
	d.mu.Unlock()
	if allocCtx == nil {
		return errBrowserClosed
	}
	tabCtx, cancel := chromedp.NewContext(allocCtx)
	if err := primeManagedTabContext(tabCtx); err != nil {
		cancel()
		return err
	}
	d.attachManagedTab(tabCtx, cancel)
	if err := d.navigateManaged(url); err != nil {
		d.mu.Lock()
		d.cancelTab = nil
		d.tabCtx = nil
		d.mu.Unlock()
		cancel()
		return err
	}
	return nil
}

func (d *daemonState) closeTab(index int) error {
	_ = index
	d.mu.Lock()
	cancelTab := d.cancelTab
	allocCtx := d.allocCtx
	d.cancelTab = nil
	d.tabCtx = nil
	d.currentURL = ""
	d.managedTarget = ""
	d.mu.Unlock()
	if allocCtx == nil {
		return errBrowserClosed
	}
	if cancelTab != nil {
		cancelTab()
	}
	return nil
}

func (d *daemonState) closeBrowser() error {
	logs.Info("chrome src_v3 closing browser intentionally")
	d.mu.Lock()
	pid := d.browserPID
	profileDir := d.profileDir
	tabCtx := d.tabCtx
	allocCtx := d.allocCtx
	cancelAlloc := d.cancelAlloc
	cancelTab := d.cancelTab
	d.intentionalStop = true
	d.unexpectedErr = nil
	d.mu.Unlock()

	gracefulErr := gracefulCloseBrowserFunc(tabCtx, allocCtx)
	if pid > 0 {
		if waitErr := waitForBrowserPIDExitFunc(pid, gracefulBrowserExitWaitTimeout); waitErr == nil {
			gracefulErr = nil
		} else {
			if gracefulErr != nil {
				logs.Warn("chrome src_v3 Browser.close did not complete for pid=%d: %v", pid, gracefulErr)
			} else {
				logs.Warn("chrome src_v3 browser pid=%d still running after graceful close wait: %v", pid, waitErr)
			}
			if err := killBrowserPIDFunc(pid); err != nil {
				logs.Error("chrome src_v3 killPID %d failed: %v", pid, err)
			}
		}
	}
	if cancelTab != nil {
		cancelTab()
	}
	if cancelAlloc != nil {
		cancelAlloc()
	}
	if runtime.GOOS == "windows" {
		if err := cleanupProfileLocksFunc(profileDir); err != nil {
			logs.Warn("chrome src_v3 cleanup profile locks failed for %s: %v", profileDir, err)
		}
	}

	d.mu.Lock()
	d.allocCtx = nil
	d.cancelAlloc = nil
	d.tabCtx = nil
	d.cancelTab = nil
	d.managedTarget = ""
	d.browserWS = ""
	d.currentURL = ""
	d.consoleLines = nil
	d.browserPID = 0
	d.mu.Unlock()

	persistDaemonStateFunc(d)
	return nil
}

func gracefulCloseBrowser(tabCtx, allocCtx context.Context) error {
	if err := gracefulCloseBrowserContext(tabCtx); err == nil {
		return nil
	}
	if allocCtx == nil {
		return errBrowserClosed
	}
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	return gracefulCloseBrowserContext(ctx)
}

func gracefulCloseBrowserContext(ctx context.Context) error {
	if ctx == nil {
		return errBrowserClosed
	}
	chromeCtx := chromedp.FromContext(ctx)
	if chromeCtx == nil || chromeCtx.Browser == nil {
		return errBrowserClosed
	}
	closeCtx, cancel := context.WithTimeout(ctx, gracefulBrowserCloseCommandTimeout)
	defer cancel()
	return cdbrowser.Close().Do(cdp.WithExecutor(closeCtx, chromeCtx.Browser))
}

func (d *daemonState) resetSession() error {
	d.mu.Lock()
	d.unexpectedErr = nil
	d.consoleLines = nil
	d.mu.Unlock()
	if err := d.ensureBrowser(); err != nil {
		return err
	}
	if err := d.navigateManaged("about:blank"); err != nil {
		return err
	}
	d.persistState()
	return nil
}

func (d *daemonState) listTabs() ([]pageInfo, error) {
	d.mu.Lock()
	pid := d.browserPID
	tabCtx := d.tabCtx
	currentURL := d.currentURL
	d.mu.Unlock()
	if pid == 0 || tabCtx == nil {
		return nil, errBrowserClosed
	}
	url := currentURL
	if liveURL, err := d.readManagedURL(); err == nil && strings.TrimSpace(liveURL) != "" {
		url = strings.TrimSpace(liveURL)
		d.mu.Lock()
		d.currentURL = url
		d.mu.Unlock()
	}
	if strings.TrimSpace(url) == "" {
		url = "about:blank"
	}
	return []pageInfo{{ID: "managed", URL: url}}, nil
}

func (d *daemonState) currentURLFromTabs(tabs []pageInfo) string {
	if len(tabs) > 0 {
		return tabs[0].URL
	}
	return ""
}

func (d *daemonState) appendConsoleLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.consoleLines = append(d.consoleLines, line)
	if len(d.consoleLines) > 200 {
		d.consoleLines = append([]string(nil), d.consoleLines[len(d.consoleLines)-200:]...)
	}
}

func (d *daemonState) consoleSnapshot() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.consoleLines...)
}

func consoleArgText(arg *cdruntime.RemoteObject) string {
	if arg == nil {
		return ""
	}
	if len(arg.Value) > 0 {
		var decoded any
		if err := json.Unmarshal(arg.Value, &decoded); err == nil {
			return strings.TrimSpace(fmt.Sprint(decoded))
		}
		return strings.TrimSpace(string(arg.Value))
	}
	if arg.UnserializableValue != "" {
		return strings.TrimSpace(string(arg.UnserializableValue))
	}
	if strings.TrimSpace(arg.Description) != "" {
		return strings.TrimSpace(arg.Description)
	}
	return strings.TrimSpace(string(arg.Type))
}
