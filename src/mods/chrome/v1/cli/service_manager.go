package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

func newChromeServiceManager(opts serverOptions) (*chromeServiceManager, error) {
	initialURL := strings.TrimSpace(opts.initialURL)
	if initialURL == "" {
		initialURL = "about:blank"
	}

	sess, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: opts.chromeDebugPort,
		GPU:           true,
		Headless:      opts.headless,
		TargetURL:     "about:blank",
		Role:          "chrome-v1-service",
		ReuseExisting: false,
	})
	if err != nil {
		return nil, fmt.Errorf("start chrome session: %w", err)
	}

	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(context.Background(), sess.WebSocketURL)
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	if err := chromedp.Run(browserCtx); err != nil {
		cancelBrowser()
		cancelAlloc()
		_ = chrome.CleanupSession(sess)
		return nil, fmt.Errorf("initialize browser connection: %w", err)
	}
	chromeCtx := chromedp.FromContext(browserCtx)
	if chromeCtx == nil || chromeCtx.Browser == nil || chromeCtx.Target == nil {
		cancelBrowser()
		cancelAlloc()
		_ = chrome.CleanupSession(sess)
		return nil, fmt.Errorf("initialize browser connection: browser or target executor missing")
	}

	mgr := &chromeServiceManager{
		session:       sess,
		allocCtx:      allocCtx,
		cancelAlloc:   cancelAlloc,
		browserCtx:    browserCtx,
		cancelBrowser: cancelBrowser,
		browser:       chromeCtx.Browser,
		headless:      opts.headless,
		tabs:          map[string]*managedTab{},
	}

	mainTab := &managedTab{
		ctx:      browserCtx,
		cancel:   cancelBrowser,
		targetID: string(chromeCtx.Target.TargetID),
	}
	if err := mgr.runOnTab(mainTab, 40*time.Second, chromedp.Navigate(initialURL)); err != nil {
		mgr.Close()
		return nil, fmt.Errorf("bootstrap main tab: %w", err)
	}
	mgr.mu.Lock()
	mgr.tabs["main"] = mainTab
	mgr.mu.Unlock()

	if err := mgr.activateTarget(target.ID(mainTab.targetID)); err != nil {
		mgr.Close()
		return nil, fmt.Errorf("activate main tab: %w", err)
	}

	targets, err := chromedp.Targets(mgr.browserCtx)
	if err == nil {
		for _, t := range targets {
			if t.Type == "page" && t.TargetID != target.ID(mainTab.targetID) {
				fmt.Printf("closing extra auto-created target: %s (%s)\n", t.TargetID, t.URL)
				_ = mgr.closeTarget(t.TargetID)
			}
		}
	}

	return mgr, nil
}

func (m *chromeServiceManager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	for _, tab := range m.tabs {
		tab.cancel()
	}
	m.tabs = map[string]*managedTab{}
	m.mu.Unlock()
	m.cancelBrowser()
	m.cancelAlloc()
	_ = chrome.CleanupSession(m.session)
}

func (m *chromeServiceManager) addTab(name, url string) (tabInfo, error) {
	tabName := normalizeTabName(name)
	if tabName == "" {
		return tabInfo{}, fmt.Errorf("tab is required")
	}
	if tabName == "main" {
		return tabInfo{}, fmt.Errorf("main tab already exists")
	}
	targetURL := strings.TrimSpace(url)
	if targetURL == "" {
		targetURL = "about:blank"
	}

	m.mu.Lock()
	if _, exists := m.tabs[tabName]; exists {
		m.mu.Unlock()
		return tabInfo{}, fmt.Errorf("tab %q already exists", tabName)
	}
	m.mu.Unlock()

	tab, err := m.newManagedTab(tabName, targetURL)
	if err != nil {
		return tabInfo{}, err
	}

	m.mu.Lock()
	m.tabs[tabName] = tab
	m.mu.Unlock()
	return tabInfo{Name: tabName, PID: m.session.PID, Headless: m.headless, TargetID: tab.targetID}, nil
}

func (m *chromeServiceManager) newManagedTab(name, url string) (*managedTab, error) {
	targetURL := strings.TrimSpace(url)
	if targetURL == "" {
		targetURL = "about:blank"
	}

	tabCtx, cancel := chromedp.NewContext(m.browserCtx)
	tab := &managedTab{
		ctx:    tabCtx,
		cancel: cancel,
	}
	if err := m.runOnTab(tab, 40*time.Second, chromedp.Navigate(targetURL)); err != nil {
		cancel()
		return nil, err
	}
	chromeCtx := chromedp.FromContext(tabCtx)
	if chromeCtx == nil || chromeCtx.Target == nil {
		cancel()
		return nil, fmt.Errorf("tab %q did not attach to a target", name)
	}
	tab.targetID = string(chromeCtx.Target.TargetID)
	return tab, nil
}

func (m *chromeServiceManager) closeTarget(id target.ID) error {
	if id == "" {
		return nil
	}
	if m.browser == nil {
		return fmt.Errorf("browser connection not initialized")
	}
	return target.CloseTarget(id).Do(cdp.WithExecutor(m.allocCtx, m.browser))
}

func (m *chromeServiceManager) activateTarget(id target.ID) error {
	if id == "" {
		return nil
	}
	if m.browser == nil {
		return fmt.Errorf("browser connection not initialized")
	}
	return target.ActivateTarget(id).Do(cdp.WithExecutor(m.allocCtx, m.browser))
}

func (m *chromeServiceManager) closeTab(name string) error {
	tabName := normalizeTabName(name)
	if tabName == "" {
		return fmt.Errorf("tab is required")
	}
	if tabName == "main" {
		return fmt.Errorf("main tab cannot be closed")
	}

	m.mu.Lock()
	tab, ok := m.tabs[tabName]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("tab %q not found", tabName)
	}
	delete(m.tabs, tabName)
	m.mu.Unlock()
	tab.cancel()
	_ = m.closeTarget(target.ID(tab.targetID))
	return nil
}

func (m *chromeServiceManager) gotoTab(name, url string) error {
	tabName := normalizeTabName(name)
	if tabName == "" {
		return fmt.Errorf("tab is required")
	}
	targetURL := strings.TrimSpace(url)
	if targetURL == "" {
		return fmt.Errorf("url is required")
	}

	for i := 0; i < 2; i++ {
		m.mu.RLock()
		tab, ok := m.tabs[tabName]
		m.mu.RUnlock()
		if !ok {
			return fmt.Errorf("tab %q not found", tabName)
		}

		err := m.runOnTab(tab, 40*time.Second, chromedp.Navigate(targetURL))
		if err == nil {
			return m.activateTarget(target.ID(tab.targetID))
		}
		if errors.Is(err, context.Canceled) && i == 0 {
			fmt.Printf("gotoTab context canceled, retrying with potentially new tab: tab=%q\n", tabName)
			continue
		}

		fmt.Printf("gotoTab recovery triggered: tab=%q error=%v\n", tabName, err)
		break
	}

	recoveredTab, recoverErr := m.newManagedTab(tabName, targetURL)
	if recoverErr != nil {
		return recoverErr
	}

	m.mu.Lock()
	old := m.tabs[tabName]
	m.tabs[tabName] = recoveredTab
	m.mu.Unlock()
	if old != nil {
		old.cancel()
		_ = m.closeTarget(target.ID(old.targetID))
	}
	return m.activateTarget(target.ID(recoveredTab.targetID))
}

func (m *chromeServiceManager) runOnTab(tab *managedTab, timeout time.Duration, actions ...chromedp.Action) error {
	tab.mu.Lock()
	defer tab.mu.Unlock()

	fmt.Printf("runOnTab: starting actions on target=%s\n", tab.targetID)
	runCtx, cancelTimeout := context.WithTimeout(tab.ctx, timeout)
	defer cancelTimeout()
	err := chromedp.Run(runCtx, actions...)
	if err != nil {
		fmt.Printf("runOnTab: target=%s error=%v\n", tab.targetID, err)
	} else {
		fmt.Printf("runOnTab: target=%s ok\n", tab.targetID)
	}
	return err
}

func (m *chromeServiceManager) devtoolsHealthy() error {
	if m == nil || m.session == nil || m.session.Port <= 0 {
		return fmt.Errorf("chrome session unavailable")
	}
	versionURL := fmt.Sprintf("http://127.0.0.1:%d/json/version", m.session.Port)
	resp, err := http.Get(versionURL) //nolint:gosec
	if err != nil {
		return fmt.Errorf("devtools unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("devtools unhealthy: status=%d", resp.StatusCode)
	}
	return nil
}

func (m *chromeServiceManager) listTabs() []tabInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]tabInfo, 0, len(m.tabs))
	for name, tab := range m.tabs {
		out = append(out, tabInfo{
			Name:     name,
			PID:      m.session.PID,
			Headless: m.headless,
			TargetID: tab.targetID,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func normalizeTabName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return ""
	}
	if idx := strings.LastIndex(name, ":"); idx >= 0 && idx < len(name)-1 {
		name = strings.TrimSpace(name[idx+1:])
	}
	return name
}
