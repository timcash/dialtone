package src_v3

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
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
		if !hasAlloc || unexpected != nil {
			if err := d.attachToRunningBrowser(); err == nil {
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
	if runtime.GOOS == "windows" {
		actualPID, err := detectBrowserPID(d.chromePort, d.role, d.profileDir)
		if err == nil && actualPID > 0 {
			logs.Info("chrome src_v3 refined browser PID: %d", actualPID)
			d.mu.Lock()
			d.browserPID = actualPID
			d.mu.Unlock()
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
	d.attachManagedTab(tabCtx, cancel)
	return nil
}

func (d *daemonState) attachManagedTab(tabCtx context.Context, cancel context.CancelFunc) {
	d.mu.Lock()
	d.tabCtx = tabCtx
	d.cancelTab = cancel
	d.managedTarget = "managed"
	d.currentURL = "about:blank"
	d.consoleLines = nil
	d.mu.Unlock()
	d.persistState()
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
	d.persistState()
	return nil
}

func (d *daemonState) resetSession() error {
	d.mu.Lock()
	d.consoleLines = nil
	d.currentURL = "about:blank"
	d.unexpectedErr = nil
	hasBrowser := d.browserPID > 0 && d.isBrowserAlive(d.browserPID, d.chromePort)
	hasAlloc := d.allocCtx != nil
	d.mu.Unlock()
	if hasBrowser && hasAlloc {
		if err := d.recreateManagedTab(); err == nil {
			d.persistState()
			return nil
		}
	}
	if hasBrowser {
		if err := d.attachToRunningBrowser(); err == nil {
			if err := d.recreateManagedTab(); err == nil {
				d.persistState()
				return nil
			}
		}
	}
	if runtime.GOOS == "windows" {
		if err := cleanupChromeProfileLocks(d.profileDir); err != nil {
			logs.Error("chrome src_v3 reset cleanup failed: %v", err)
		}
	}
	if err := d.ensureBrowser(); err != nil {
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
