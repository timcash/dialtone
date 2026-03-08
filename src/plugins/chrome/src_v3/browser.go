package src_v3

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/chromedp/cdproto/page"
	cdruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func (d *daemonState) isBrowserAlive(pid int, port int) bool {
	if pid <= 0 {
		return false
	}
	client := &http.Client{Timeout: 1000 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		if runtime.GOOS == "windows" {
			return false
		}
		proc, findErr := os.FindProcess(pid)
		if findErr != nil || proc == nil {
			return false
		}
		return proc.Signal(os.Signal(nil)) == nil
	}
	resp.Body.Close()
	if runtime.GOOS == "windows" {
		return true
	}
	proc, err := os.FindProcess(pid)
	if err != nil || proc == nil {
		return false
	}
	return proc.Signal(os.Signal(nil)) == nil
}

func (d *daemonState) ensureBrowser() error {
	d.mu.Lock()
	if d.unexpectedErr != nil {
		err := d.unexpectedErr
		d.mu.Unlock()
		return fmt.Errorf("browser connection unhealthy: %w", err)
	}
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
		logs.Error("chrome src_v3 detected browser death, marking unhealthy")
		d.mu.Lock()
		d.unexpectedErr = fmt.Errorf("browser process or port lost")
		err := d.unexpectedErr
		d.mu.Unlock()
		return fmt.Errorf("browser connection unhealthy: %w", err)
	}
	d.mu.Unlock()

	if runtime.GOOS == "windows" {
		if err := cleanupChromeProfileLocks(d.profileDir); err != nil {
			logs.Error("chrome src_v3 failed to clean profile locks: %v", err)
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
	if actualPID, err := detectBrowserPID(d.chromePort, d.role, d.profileDir); err == nil && actualPID > 0 {
		logs.Info("chrome src_v3 refined browser PID: %d", actualPID)
		d.mu.Lock()
		d.browserPID = actualPID
		d.mu.Unlock()
	} else if runtime.GOOS == "windows" {
		_ = killPID(pid)
		return fmt.Errorf("failed to detect real chrome browser process on port %d", d.chromePort)
	}

	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	d.mu.Lock()
	d.allocCtx = allocCtx
	d.cancelAlloc = cancel
	d.browserWS = wsURL
	d.mu.Unlock()

	go func() {
		<-allocCtx.Done()
		d.mu.Lock()
		if !d.intentionalStop && d.unexpectedErr == nil {
			logs.Error("chrome src_v3 allocator connection closed")
			d.unexpectedErr = fmt.Errorf("browser allocator connection lost")
		}
		d.mu.Unlock()
	}()

	return d.ensureManagedTab()
}

func (d *daemonState) ensureManagedTab() error {
	d.mu.Lock()
	if d.tabCtx != nil {
		d.mu.Unlock()
		return nil
	}
	allocCtx := d.allocCtx
	d.mu.Unlock()
	if allocCtx == nil {
		return errBrowserClosed
	}
	tabCtx, cancel := chromedp.NewContext(allocCtx)
	err := chromedp.Run(tabCtx)
	if err != nil {
		cancel()
		return err
	}
	d.mu.Lock()
	d.tabCtx = tabCtx
	d.cancelTab = cancel
	d.managedTarget = "managed"
	d.currentURL = "about:blank"
	d.consoleLines = nil
	d.mu.Unlock()
	chromedp.ListenTarget(tabCtx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *cdruntime.EventConsoleAPICalled:
			parts := make([]string, 0, len(ev.Args))
			for _, arg := range ev.Args {
				parts = append(parts, string(arg.Value))
			}
			d.appendConsoleLine(strings.TrimSpace(strings.Join(parts, " ")))
		case *cdruntime.EventExceptionThrown:
			d.appendConsoleLine(strings.TrimSpace(ev.ExceptionDetails.Text))
		}
	})
	return nil
}

func (d *daemonState) recreateManagedTab() error {
	d.mu.Lock()
	cancelTab := d.cancelTab
	d.cancelTab = nil
	d.tabCtx = nil
	d.currentURL = "about:blank"
	d.managedTarget = "managed"
	d.mu.Unlock()
	if cancelTab != nil {
		cancelTab()
	}
	return d.ensureManagedTab()
}

func (d *daemonState) startBrowserProcess() (int, error) {
	if runtime.GOOS == "windows" {
		return startDetachedWindowsProcess(d.chromePath, d.browserArgs())
	}
	cmd := exec.Command(d.chromePath, d.browserArgs()...)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

func (d *daemonState) browserArgs() []string {
	return []string{
		fmt.Sprintf("--remote-debugging-port=%d", d.chromePort),
		"--remote-debugging-address=127.0.0.1",
		"--remote-allow-origins=*",
		"--user-data-dir=" + d.profileDir,
		"--dialtone-role=" + d.role,
		"--dialtone-managed-profile=" + d.profileDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-gpu",
		"about:blank",
	}
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

func (d *daemonState) navigateManaged(rawURL string) error {
	url := normalizeURL(rawURL)
	return d.withManagedContext(30*time.Second, func(ctx context.Context) error {
		if err := chromedp.Run(ctx, chromedp.Navigate(url)); err != nil {
			return err
		}
		d.mu.Lock()
		d.currentURL = url
		d.mu.Unlock()
		return nil
	})
}

func (d *daemonState) clickAriaLabel(label string) error {
	selector := ariaSelector(label)
	return d.withManagedContext(15*time.Second, func(ctx context.Context) error {
		return chromedp.Run(ctx,
			chromedp.WaitVisible(selector, chromedp.ByQuery),
			chromedp.Click(selector, chromedp.ByQuery),
		)
	})
}

func (d *daemonState) pressEnterAriaLabel(label string) error {
	selector := ariaSelector(label)
	return d.withManagedContext(15*time.Second, func(ctx context.Context) error {
		return chromedp.Run(ctx,
			chromedp.WaitVisible(selector, chromedp.ByQuery),
			chromedp.SendKeys(selector, "\r", chromedp.ByQuery),
		)
	})
}

func (d *daemonState) typeAriaLabel(label, value string) error {
	selector := ariaSelector(label)
	selectorJSON, err := json.Marshal(selector)
	if err != nil {
		return err
	}
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return err
	}
	script := fmt.Sprintf(`(() => {
		const el = document.querySelector(%s);
		if (!el) return "missing";
		el.focus();
		el.value = %s;
		el.dispatchEvent(new Event("input", { bubbles: true }));
		el.dispatchEvent(new Event("change", { bubbles: true }));
		return "ok";
	})()`, string(selectorJSON), string(valueJSON))
	return d.withManagedContext(15*time.Second, func(ctx context.Context) error {
		var result string
		if err := chromedp.Run(ctx,
			chromedp.WaitVisible(selector, chromedp.ByQuery),
			chromedp.Evaluate(script, &result),
		); err != nil {
			return err
		}
		if result != "ok" {
			return fmt.Errorf("type target %q not found", label)
		}
		return nil
	})
}

func (d *daemonState) waitForAriaLabel(label string, timeout time.Duration) error {
	selector := ariaSelector(label)
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return d.withManagedContext(timeout, func(ctx context.Context) error {
		return chromedp.Run(ctx, chromedp.WaitVisible(selector, chromedp.ByQuery))
	})
}

func (d *daemonState) waitForAriaLabelAttrEquals(label, attr, expected string, timeout time.Duration) error {
	if strings.TrimSpace(attr) == "" {
		return fmt.Errorf("wait-aria-attr requires attr")
	}
	selector := ariaSelector(label)
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return d.withManagedContext(timeout, func(ctx context.Context) error {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			var actual string
			var ok bool
			if err := chromedp.Run(ctx, chromedp.AttributeValue(selector, attr, &actual, &ok, chromedp.ByQuery)); err == nil && ok && actual == expected {
				return nil
			}
			time.Sleep(120 * time.Millisecond)
		}
		return fmt.Errorf("timed out waiting for aria-label %q attr %q=%q", label, attr, expected)
	})
}

func (d *daemonState) setManagedHTML(markup string) error {
	if err := d.navigateManaged("about:blank"); err != nil {
		return err
	}
	raw, err := json.Marshal(markup)
	if err != nil {
		return err
	}
	script := fmt.Sprintf(`document.open(); document.write(%s); document.close();`, string(raw))
	return d.withManagedContext(15*time.Second, func(ctx context.Context) error {
		return chromedp.Run(ctx, chromedp.Evaluate(script, nil))
	})
}

func (d *daemonState) waitForConsoleContains(substr string, timeout time.Duration) ([]string, error) {
	substr = strings.TrimSpace(substr)
	if substr == "" {
		return nil, fmt.Errorf("wait-log requires contains")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		d.mu.Lock()
		lines := append([]string(nil), d.consoleLines...)
		d.mu.Unlock()
		for _, line := range lines {
			if strings.Contains(line, substr) {
				return lines, nil
			}
		}
		time.Sleep(120 * time.Millisecond)
	}
	return nil, fmt.Errorf("timed out waiting for console log containing %q", substr)
}

func (d *daemonState) consoleSnapshot() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.consoleLines...)
}

func (d *daemonState) captureScreenshotB64() (string, error) {
	var buf []byte
	err := d.withManagedContext(20*time.Second, func(ctx context.Context) error {
		return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			data, err := page.CaptureScreenshot().
				WithCaptureBeyondViewport(false).
				WithFromSurface(true).
				Do(ctx)
			if err != nil {
				return err
			}
			buf = data
			return nil
		}))
	})
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf), nil
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
	d.mu.Lock()
	d.tabCtx = tabCtx
	d.cancelTab = cancel
	d.managedTarget = "managed"
	d.mu.Unlock()
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
	d.mu.Lock()
	defer d.mu.Unlock()
	logs.Info("chrome src_v3 closing browser intentionally")
	d.intentionalStop = true
	if d.cancelAlloc != nil {
		d.cancelAlloc()
	}
	if d.cancelTab != nil {
		d.cancelTab()
	}
	d.allocCtx = nil
	d.cancelAlloc = nil
	d.tabCtx = nil
	d.cancelTab = nil
	d.managedTarget = ""
	d.browserWS = ""
	d.currentURL = ""
	d.unexpectedErr = nil
	if d.browserPID > 0 {
		if err := killPID(d.browserPID); err != nil {
			logs.Error("chrome src_v3 killPID %d failed: %v", d.browserPID, err)
		}
	}
	d.browserPID = 0
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

func (d *daemonState) readManagedURL() (string, error) {
	var current string
	err := d.withManagedContext(10*time.Second, func(ctx context.Context) error {
		return chromedp.Run(ctx, chromedp.Location(&current))
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(current), nil
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

func ariaSelector(label string) string {
	return fmt.Sprintf(`[aria-label=%q]`, strings.TrimSpace(label))
}

func findChromePath() (string, error) {
	if runtime.GOOS == "windows" {
		candidates := []string{
			filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
		}
		for _, candidate := range candidates {
			if strings.TrimSpace(candidate) != "" {
				if _, err := os.Stat(candidate); err == nil {
					return candidate, nil
				}
			}
		}
		return "", fmt.Errorf("chrome.exe not found")
	}
	if p, err := exec.LookPath("google-chrome"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("chromium"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("chrome not found in PATH")
}

func waitForWebSocket(port int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
		if err == nil {
			var payload struct {
				WS string `json:"webSocketDebuggerUrl"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil {
				_ = resp.Body.Close()
				if strings.TrimSpace(payload.WS) != "" {
					return strings.TrimSpace(payload.WS), nil
				}
			}
			_ = resp.Body.Close()
		}
		time.Sleep(250 * time.Millisecond)
	}
	return "", fmt.Errorf("timed out waiting for chrome debug websocket on port %d", port)
}

func detectBrowserPID(port int, role, profileDir string) (int, error) {
	switch runtime.GOOS {
	case "windows":
		for i := 0; i < 5; i++ {
			script := fmt.Sprintf(`$port=%d; `+
				`$listener=$null; `+
				`try { $listener=Get-NetTCPConnection -LocalAddress '127.0.0.1' -LocalPort $port -State Listen -ErrorAction Stop | Select-Object -First 1 -ExpandProperty OwningProcess } catch {}; `+
				`if(-not $listener){ try { $listener=Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction Stop | Select-Object -First 1 -ExpandProperty OwningProcess } catch {} }; `+
				`if($listener){ Write-Output $listener; exit 0 }; `+
				`$role=%s; $profile=%s; `+
				`$procs=Get-CimInstance Win32_Process | Where-Object { $_.Name -eq 'chrome.exe' -and $_.CommandLine -like ('*--remote-debugging-port=' + $port + '*') -and ($_.CommandLine -like ('*--dialtone-role=' + $role + '*') -or $_.CommandLine -like ('*' + $profile + '*')) } | Select-Object -First 1 -ExpandProperty ProcessId; `+
				`if($procs){ Write-Output $procs }`, port, psQuote(role), psQuote(windowsPath(profileDir)))
			out, err := exec.Command("powershell", "-NoProfile", "-Command", script).CombinedOutput()
			if err == nil {
				if n, convErr := strconv.Atoi(strings.TrimSpace(string(out))); convErr == nil && n > 0 {
					return n, nil
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
		return 0, fmt.Errorf("chrome pid not found for port %d", port)
	default:
		out, err := exec.Command("bash", "-lc", fmt.Sprintf("ps -eo pid,args | grep '[c]hrome' | grep -- '--remote-debugging-port=%d' | grep -- '--dialtone-role=%s' | head -n1 | awk '{print $1}'", port, shellEscapeGrep(role))).Output()
		if err != nil {
			return 0, err
		}
		n, err := strconv.Atoi(strings.TrimSpace(string(out)))
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("chrome pid not found")
		}
		return n, nil
	}
}

func cleanupChromeProfileLocks(profileDir string) error {
	lockNames := []string{"SingletonLock", "SingletonCookie", "SingletonSocket"}
	var errs []string
	for _, name := range lockNames {
		path := filepath.Join(profileDir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	return nil
}

func startDetachedWindowsProcess(exePath string, args []string) (int, error) {
	quotedArgs := make([]string, 0, len(args))
	for _, arg := range args {
		quotedArgs = append(quotedArgs, psQuote(arg))
	}
	script := fmt.Sprintf("$p = Start-Process -FilePath %s -ArgumentList @(%s) -WindowStyle Hidden -PassThru; $p.Id",
		psQuote(windowsPath(exePath)),
		strings.Join(quotedArgs, ","),
	)
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("start detached chrome failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(string(out)))
	if convErr != nil {
		return 0, fmt.Errorf("unable to parse detached chrome pid from %q: %w", strings.TrimSpace(string(out)), convErr)
	}
	return pid, nil
}

func killPID(pid int) error {
	if pid <= 0 {
		return nil
	}
	if runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run()
	}
	return exec.Command("kill", "-9", strconv.Itoa(pid)).Run()
}

func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") ||
		strings.HasPrefix(raw, "about:") ||
		strings.HasPrefix(raw, "data:") ||
		strings.HasPrefix(raw, "file:") {
		return raw
	}
	return "https://" + raw
}
