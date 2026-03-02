package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
)

func handleOpenCmd(args []string) {
	fs := flag.NewFlagSet("chrome open", flag.ExitOnError)
	host := fs.String("host", "", "Target host name, or 'all'")
	url := fs.String("url", "about:blank", "URL to open")
	fullscreen := fs.Bool("fullscreen", false, "Set browser window fullscreen after navigate")
	kiosk := fs.Bool("kiosk", false, "Launch or reuse in kiosk mode (headed only)")
	servicePort := fs.Int("service-port", defaultChromeServicePort, "Remote chrome service command port")
	role := fs.String("role", "dev", "Role tag to reuse/start")
	retries := fs.Int("retries", 3, "Retry attempts per host when wake/sleep causes transient failures")
	retryDelay := fs.Duration("retry-delay", 2*time.Second, "Delay between retry attempts")
	_ = fs.Parse(args)

	target := strings.TrimSpace(*host)
	if target == "" {
		logs.Fatal("open requires --host")
	}
	targetURL := normalizeOpenURL(strings.TrimSpace(*url))
	if *kiosk {
		*fullscreen = true
	}

	nodes, err := resolveChromeHosts(target)
	if err != nil {
		logs.Fatal("open --host: %v", err)
	}

	ok := 0
	fail := 0
	for _, node := range nodes {
		t := strings.TrimSpace(node.Name)
		var wsURL string
		var err error
		attempts := *retries
		if attempts < 1 {
			attempts = 1
		}
		for i := 1; i <= attempts; i++ {
			wsURL, err = openOnHost(t, targetURL, strings.TrimSpace(*role), *servicePort, *fullscreen, *kiosk)
			if err == nil {
				break
			}
			if i < attempts {
				logs.Warn("open host=%s attempt=%d/%d failed: %v", t, i, attempts, err)
				time.Sleep(*retryDelay)
			}
		}
		if err != nil {
			fail++
			logs.Warn("open host=%s failed after %d attempt(s): %v", t, attempts, err)
			continue
		}
		if strings.EqualFold(strings.TrimSpace(t), "local") {
			if err := navigateAndFullscreen(wsURL, targetURL, *fullscreen); err != nil {
				// Keep the session as success: open/reuse is already complete; control is best-effort.
				logs.Warn("open host=%s control warning: %v", t, err)
			}
		}
		ok++
		logs.Info("open host=%s ok ws=%s", t, strings.TrimSpace(wsURL))
	}
	if ok == 0 {
		logs.Fatal("open failed on all targets (%d)", fail)
	}
	if fail > 0 {
		logs.Warn("open completed with partial failures: ok=%d fail=%d", ok, fail)
	} else {
		logs.Info("open completed: ok=%d", ok)
	}
}

func openOnHost(host, targetURL, role string, servicePort int, fullscreen bool, kiosk bool) (string, error) {
	if strings.EqualFold(host, "local") {
		sess, err := chrome.StartSession(chrome.SessionOptions{
			GPU:           true,
			Headless:      false,
			Kiosk:         kiosk,
			TargetURL:     targetURL,
			Role:          role,
			ReuseExisting: true,
		})
		if err != nil {
			return "", err
		}
		if err := ensureSinglePageTab(sess.Port); err != nil {
			return "", err
		}
		return strings.TrimSpace(sess.WebSocketURL), nil
	}
	if ws, err := requestRemoteServiceOpen(strings.TrimSpace(host), servicePort, openRequest{debugURLRequest: debugURLRequest{
		Role:         role,
		Headless:     false,
		URL:          targetURL,
		Reuse:        true,
		DebugAddress: "0.0.0.0",
	}, Fullscreen: fullscreen, Kiosk: kiosk}); err == nil && strings.TrimSpace(ws) != "" {
		return strings.TrimSpace(ws), nil
	}
	return "", fmt.Errorf("remote chrome service unavailable on %s", strings.TrimSpace(host))
}

func navigateAndFullscreen(wsURL, targetURL string, fullscreen bool) error {
	attachWS := strings.TrimSpace(wsURL)
	if resolved, err := resolveExistingPageWebSocket(attachWS); err == nil && strings.TrimSpace(resolved) != "" {
		attachWS = strings.TrimSpace(resolved)
	}
	ctx, cancel, err := chrome.AttachToWebSocket(attachWS)
	if err != nil {
		return err
	}
	defer cancel()

	deadlineCtx, deadlineCancel := context.WithTimeout(ctx, 15*time.Second)
	defer deadlineCancel()

	if err := chromedp.Run(deadlineCtx, chromedp.Navigate(targetURL)); err != nil {
		return err
	}
	if !fullscreen {
		return nil
	}

	winID, _, err := browser.GetWindowForTarget().Do(deadlineCtx)
	if err != nil {
		return err
	}
	bounds := &browser.Bounds{WindowState: browser.WindowStateFullscreen}
	return browser.SetWindowBounds(winID, bounds).Do(deadlineCtx)
}

func resolveExistingPageWebSocket(rawWS string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawWS))
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	listURL := fmt.Sprintf("http://%s/json/list", strings.TrimSpace(u.Host))
	resp, err := client.Get(listURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("json/list status=%s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	type targetInfo struct {
		Type string `json:"type"`
		WS   string `json:"webSocketDebuggerUrl"`
	}
	var targets []targetInfo
	if err := json.Unmarshal(body, &targets); err != nil {
		return "", err
	}
	for _, t := range targets {
		if !strings.EqualFold(strings.TrimSpace(t.Type), "page") {
			continue
		}
		pageWS := strings.TrimSpace(t.WS)
		if pageWS == "" {
			continue
		}
		return rewritePageWebSocketForAccess(pageWS, u), nil
	}
	return "", fmt.Errorf("no page target in json/list")
}

func normalizeOpenURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "about:blank"
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "about:") {
		return raw
	}
	return "https://" + raw
}

func rewritePageWebSocketForAccess(pageWS string, serviceURL *url.URL) string {
	rawWS := strings.TrimSpace(pageWS)
	if serviceURL == nil {
		return rawWS
	}
	serviceHost := strings.TrimSpace(serviceURL.Hostname())
	servicePort, _ := strconv.Atoi(strings.TrimSpace(serviceURL.Port()))
	if servicePort <= 0 {
		servicePort = defaultChromeServicePort
	}
	return normalizeWSHost(rawWS, serviceHost, servicePort)
}

func normalizeWSHost(rawWS, host string, servicePort int) string {
	rawWS = strings.TrimSpace(rawWS)
	host = strings.TrimSpace(host)
	if rawWS == "" || host == "" {
		return rawWS
	}
	u, err := url.Parse(rawWS)
	if err != nil {
		return rawWS
	}
	wsHost := strings.TrimSpace(u.Hostname())
	if !isLoopbackHost(wsHost) {
		return rawWS
	}
	port, _ := strconv.Atoi(strings.TrimSpace(u.Port()))
	if port <= 0 {
		return rawWS
	}
	if !canDialHost(host, port, 700*time.Millisecond) && servicePort > 0 {
		u.Host = fmt.Sprintf("%s:%d", host, servicePort)
		return u.String()
	}
	u.Host = fmt.Sprintf("%s:%d", host, port)
	return u.String()
}

func isLoopbackHost(h string) bool {
	h = strings.ToLower(strings.TrimSpace(h))
	return h == "127.0.0.1" || h == "localhost" || h == "::1"
}
