package test

import (
	"fmt"
	"strings"

	chrome "dialtone/dev/plugins/chrome/src_v3"
)

func StartBrowser(opts BrowserOptions) (*BrowserSession, error) {
	navigateOnStart := strings.TrimSpace(opts.URL) != "" && !opts.SkipNavigateOnReuse && !opts.PreserveTabAndSize
	remoteNode := resolveRemoteBrowserNode(opts)
	if remoteNode == "" {
		return nil, fmt.Errorf("browser service host is required; set --attach and start chrome src_v3 on that host")
	}
	cfg := RuntimeConfigSnapshot()
	role := strings.TrimSpace(opts.Role)
	if remoteNode != "" && strings.TrimSpace(cfg.RemoteBrowserRole) != "" {
		role = strings.TrimSpace(cfg.RemoteBrowserRole)
	}
	resp, err := chrome.SendCommandByHost(remoteNode, chrome.CommandRequest{
		Command: "status",
		Role:    role,
	})
	if err != nil {
		return nil, err
	}
	if resp.BrowserPID == 0 {
		resp, err = chrome.SendCommandByHost(remoteNode, chrome.CommandRequest{
			Command: "open",
			Role:    role,
			URL:     "about:blank",
		})
		if err != nil {
			return nil, err
		}
	}
	s, err := initSession(chrome.NewSessionFromResponse(remoteNode, resp), role)
	if err != nil {
		return nil, err
	}
	if navigateOnStart {
		if err := s.Navigate(opts.URL); err != nil {
			s.Close()
			return nil, err
		}
	}
	return s, nil
}

func resolveRemoteBrowserNode(opts BrowserOptions) string {
	if node := strings.TrimSpace(opts.RemoteNode); node != "" {
		return node
	}
	return strings.TrimSpace(RuntimeConfigSnapshot().BrowserNode)
}

func startRemoteBrowser(node string, opts BrowserOptions) (*BrowserSession, error) {
	opts.RemoteNode = node
	return StartBrowser(opts)
}

func startBrowserViaChromeCLI(opts BrowserOptions, navigateOnStart bool) (*BrowserSession, error) {
	return nil, fmt.Errorf("chrome cli session bootstrap retired; use chrome acquire session")
}

func probeRequestedDebugPort(port int) (bool, bool) {
	return false, false
}

func findAttachableDialtoneSession(role string, headless bool) *chrome.Session {
	return nil
}
