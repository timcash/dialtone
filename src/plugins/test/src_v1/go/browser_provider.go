package test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	stdruntime "runtime"
	"strings"
	"time"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/chromedp/chromedp"
)

func StartBrowser(opts BrowserOptions) (*BrowserSession, error) {
	navigateOnStart := strings.TrimSpace(opts.URL) != "" && !opts.SkipNavigateOnReuse && !opts.PreserveTabAndSize
	if s, err := startBrowserViaChromeCLI(opts, navigateOnStart); err == nil && s != nil {
		return s, nil
	}
	if remoteNode := resolveRemoteBrowserNode(opts); remoteNode != "" {
		logs.Info("   [BROWSER] Remote node configured; trying remote-first on %s", remoteNode)
		rs, rerr := startRemoteBrowser(remoteNode, opts)
		if rerr == nil {
			if navigateOnStart {
				if err := chromedp.Run(rs.ctx, chromedp.Navigate(opts.URL)); err != nil {
					if opts.ReuseExisting {
						errText := strings.ToLower(err.Error())
						if strings.Contains(errText, "no browser is open") || strings.Contains(errText, "failed to open new tab") {
							if epErr := rs.EnsureOpenPage(); epErr == nil {
								if nerr := chromedp.Run(rs.ctx, chromedp.Navigate(opts.URL)); nerr == nil {
									return rs, nil
								}
							}
						}
						// Keep the currently attached headed browser; do not spawn/attach again.
						logs.Warn("   [BROWSER] remote reused session navigate failed on %s; continuing with existing attached session", remoteNode)
						return rs, nil
					}
					rs.Close()
					return nil, err
				}
			}
			return rs, nil
		}
		if strings.TrimSpace(opts.RemoteNode) != "" {
			return nil, fmt.Errorf("remote-first failed on %s: %w", remoteNode, rerr)
		}
		if RuntimeConfigSnapshot().NoSSH {
			logs.Warn("   [BROWSER] remote-first failed on %s in no-ssh mode; not falling back to local", remoteNode)
			return nil, fmt.Errorf("remote-first failed on %s in no-ssh mode: %w", remoteNode, rerr)
		}
		logs.Warn("   [BROWSER] remote-first failed on %s; falling back to local start", remoteNode)
	}

	cfg := RuntimeConfigSnapshot()
	requestedPort := cfg.RemoteDebugPort
	debugAddress := ""
	wslMode := stdruntime.GOOS == "linux" && wslGatewayIP() != ""
	if wslMode && requestedPort <= 0 {
		switch strings.ToLower(strings.TrimSpace(opts.Role)) {
		case "dev":
			requestedPort = chrome.DefaultDebugPort
		case "test":
			requestedPort = 9223
		default:
			requestedPort = 9334
		}
	}
	if requestedPort > 0 {
		if wslMode {
			// WSL should bind/debug through the Windows host path, not loopback-only.
			debugAddress = "0.0.0.0"
		}
		isChromeDebug, isInUse := probeRequestedDebugPort(requestedPort)
		if isChromeDebug {
			logs.Info("   [BROWSER] requested debug port %d already serves Chrome DevTools; reusing it", requestedPort)
			s, err := ConnectToBrowser(requestedPort, opts.Role)
			if err == nil {
				if navigateOnStart {
					if err := chromedp.Run(s.ctx, chromedp.Navigate(opts.URL)); err != nil {
						s.Close()
						return nil, err
					}
				}
				return s, nil
			}
			logs.Warn("   [BROWSER] reuse existing debug port %d failed; falling back to launch path: %v", requestedPort, err)
		} else if isInUse && !opts.ReuseExisting {
			if wslMode {
				logs.Warn("   [BROWSER] requested debug port %d is in use (likely WSL proxy listener); keeping it and launching Chrome on loopback", requestedPort)
			} else {
				logs.Warn("   [BROWSER] requested debug port %d in use by non-Chrome endpoint; cleaning it before launch", requestedPort)
				if err := chrome.CleanupPort(requestedPort); err != nil {
					return nil, fmt.Errorf("cleanup occupied requested debug port %d: %w", requestedPort, err)
				}
			}
		}
	}

	logs.Info("   [BROWSER] Starting session (role=%s, reuse=%v, gpu=%v)...", opts.Role, opts.ReuseExisting, opts.GPU)
	session, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: requestedPort,
		Headless:      opts.Headless,
		GPU:           opts.GPU,
		Role:          opts.Role,
		ReuseExisting: opts.ReuseExisting,
		UserDataDir:   opts.UserDataDir,
		TargetURL:     opts.URL,
		DebugAddress:  debugAddress,
	})
	if err != nil {
		// First fallback: attach to an already-running Dialtone browser for this role/headless mode.
		if attach := findAttachableDialtoneSession(opts.Role, opts.Headless); attach != nil {
			s, aerr := initSession(attach, opts.Role)
			if aerr == nil {
				if navigateOnStart {
					if navErr := chromedp.Run(s.ctx, chromedp.Navigate(opts.URL)); navErr != nil {
						s.Close()
						return nil, navErr
					}
				}
				return s, nil
			}
		}
		// Second fallback: force a fresh launch (disable reuse) after a short settle delay.
		time.Sleep(300 * time.Millisecond)
		session, err = chrome.StartSession(chrome.SessionOptions{
			RequestedPort: requestedPort,
			Headless:      opts.Headless,
			GPU:           opts.GPU,
			Role:          opts.Role,
			ReuseExisting: false,
			UserDataDir:   opts.UserDataDir,
			TargetURL:     opts.URL,
			DebugAddress:  debugAddress,
		})
		if err != nil {
			if remoteNode := resolveRemoteBrowserNode(opts); remoteNode != "" {
				logs.Warn("   [BROWSER] local launch failed; trying remote node %s", remoteNode)
				rs, rerr := startRemoteBrowser(remoteNode, opts)
				if rerr == nil {
					if navigateOnStart {
						if err := chromedp.Run(rs.ctx, chromedp.Navigate(opts.URL)); err != nil {
							rs.Close()
							return nil, err
						}
					}
					return rs, nil
				}
				return nil, fmt.Errorf("failed local start (%v), remote fallback failed on %s (%v)", err, remoteNode, rerr)
			}
			return nil, fmt.Errorf("failed to start chrome session: %w", err)
		}
	}

	s, err := initSession(session, opts.Role)
	if err != nil {
		if remoteNode := resolveRemoteBrowserNode(opts); remoteNode != "" {
			logs.Warn("   [BROWSER] local session attach failed; trying remote node %s", remoteNode)
			rs, rerr := startRemoteBrowser(remoteNode, opts)
			if rerr == nil {
				if navigateOnStart {
					if err := chromedp.Run(rs.ctx, chromedp.Navigate(opts.URL)); err != nil {
						rs.Close()
						return nil, err
					}
				}
				return rs, nil
			}
			return nil, fmt.Errorf("local session init failed (%v), remote fallback failed on %s (%v)", err, remoteNode, rerr)
		}
		return nil, err
	}

	if navigateOnStart {
		logs.Info("   [BROWSER] Navigating to: %s", opts.URL)
		if err := chromedp.Run(s.ctx, chromedp.Navigate(opts.URL)); err != nil {
			s.Close()
			if remoteNode := resolveRemoteBrowserNode(opts); remoteNode != "" {
				logs.Warn("   [BROWSER] local navigate failed; trying remote node %s", remoteNode)
				rs, rerr := startRemoteBrowser(remoteNode, opts)
				if rerr == nil {
					if navigateOnStart {
						if nerr := chromedp.Run(rs.ctx, chromedp.Navigate(opts.URL)); nerr != nil {
							rs.Close()
							return nil, nerr
						}
					}
					return rs, nil
				}
				return nil, fmt.Errorf("local navigate failed (%v), remote fallback failed on %s (%v)", err, remoteNode, rerr)
			}
			return nil, err
		}
	}

	return s, nil
}

func startBrowserViaChromeCLI(opts BrowserOptions, navigateOnStart bool) (*BrowserSession, error) {
	role := strings.TrimSpace(opts.Role)
	if role == "" {
		role = "test"
	}
	args := []string{"chrome", "src_v1", "debug-url", "--role", role}
	if opts.Headless {
		args = append(args, "--headless")
	}
	if u := strings.TrimSpace(opts.URL); u != "" {
		args = append(args, "--url", u)
	}
	if n := strings.TrimSpace(resolveRemoteBrowserNode(opts)); n != "" {
		args = append(args, "--host", n)
	}
	cmd := exec.Command("./dialtone.sh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("chrome debug-url cli failed: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	wsURL := ""
	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "ws://") || strings.HasPrefix(strings.ToLower(line), "wss://") {
			wsURL = line
		}
	}
	if wsURL == "" {
		return nil, fmt.Errorf("chrome debug-url cli returned no websocket url")
	}
	session := &chrome.Session{
		PID:          0,
		Port:         debugPortFromWebSocketURL(wsURL),
		WebSocketURL: wsURL,
		IsNew:        false,
	}
	s, err := initSession(session, role)
	if err != nil {
		return nil, err
	}
	if navigateOnStart {
		if err := chromedp.Run(s.ctx, chromedp.Navigate(opts.URL)); err != nil {
			s.Close()
			return nil, err
		}
	}
	return s, nil
}

func probeRequestedDebugPort(port int) (isChromeDebug bool, isInUse bool) {
	if port <= 0 {
		return false, false
	}
	hosts := []string{"127.0.0.1"}
	if stdruntime.GOOS == "linux" {
		if gw := wslGatewayIP(); gw != "" {
			// In WSL NAT mode, always probe the Windows host gateway and avoid localhost fallback.
			hosts = []string{gw}
		}
	}
	isReachable := false
	for _, host := range hosts {
		if canDialHostPort(host, port, 600*time.Millisecond) {
			isReachable = true
			if isChromeDevToolsEndpoint(host, port) {
				return true, true
			}
		}
	}
	return false, isReachable
}

func isChromeDevToolsEndpoint(host string, port int) bool {
	client := &http.Client{Timeout: 900 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://%s:%d/json/version", strings.TrimSpace(host), port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var data struct {
		Browser              string `json:"Browser"`
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return false
	}
	b := strings.ToLower(strings.TrimSpace(data.Browser))
	if b == "" || strings.TrimSpace(data.WebSocketDebuggerURL) == "" {
		return false
	}
	return strings.Contains(b, "chrome") || strings.Contains(b, "chromium") || strings.Contains(b, "edge")
}

func canDialHostPort(host string, port int, timeout time.Duration) bool {
	host = strings.TrimSpace(host)
	if host == "" || port <= 0 {
		return false
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func resolveRemoteBrowserNode(opts BrowserOptions) string {
	if n := strings.TrimSpace(opts.RemoteNode); n != "" {
		return n
	}
	return strings.TrimSpace(RuntimeConfigSnapshot().BrowserNode)
}

func startRemoteBrowser(node string, opts BrowserOptions) (*BrowserSession, error) {
	role := strings.TrimSpace(opts.Role)
	if role == "" {
		role = "test"
	}
	cfg := RuntimeConfigSnapshot()
	remoteRes, err := chrome.StartRemoteSession(node, chrome.RemoteSessionOptions{
		Role:               role,
		URL:                strings.TrimSpace(opts.URL),
		Headless:           opts.Headless,
		GPU:                opts.GPU,
		PreferredDebugPort: cfg.RemoteDebugPort,
		DebugPorts:         append([]int(nil), cfg.RemoteDebugPorts...),
		PreferredPID:       cfg.RemoteBrowserPID,
		RequireRole:        cfg.RemoteRequireRole,
		NoSSH:              cfg.NoSSH,
		NoLaunch:           cfg.RemoteNoLaunch,
	})
	if err != nil {
		return nil, err
	}
	s, err := initSession(remoteRes.Session, role)
	if err != nil {
		for _, closer := range remoteRes.Closers {
			if closer != nil {
				_ = closer.Close()
			}
		}
		return nil, err
	}
	if len(remoteRes.Closers) > 0 {
		s.closers = append(s.closers, remoteRes.Closers...)
	}
	return s, nil
}

func findAttachableDialtoneSession(role string, headless bool) *chrome.Session {
	procs, err := chrome.ListResources(true)
	if err != nil {
		return nil
	}
	for _, p := range procs {
		if p.Origin != "Dialtone" {
			continue
		}
		if strings.TrimSpace(role) != "" && p.Role != role {
			continue
		}
		if p.IsHeadless != headless {
			continue
		}
		if p.DebugPort <= 0 {
			continue
		}
		wsURL, err := getWebsocketURL(p.DebugPort)
		if err != nil || strings.TrimSpace(wsURL) == "" {
			continue
		}
		return &chrome.Session{
			PID:          p.PID,
			Port:         p.DebugPort,
			WebSocketURL: wsURL,
			IsNew:        false,
			IsWindows:    p.IsWindows,
		}
	}
	return nil
}
