package test

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

func IsWSLRuntime() bool {
	raw, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(raw)), "microsoft")
}

func RewriteLocalURLToWSLHost(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u == nil {
		return "", nil
	}
	host := strings.TrimSpace(u.Hostname())
	if host != "127.0.0.1" && host != "localhost" {
		return raw, nil
	}
	wsl, err := sshv1.ResolveMeshNode("wsl")
	if err != nil {
		return "", err
	}
	meshHost := strings.TrimSpace(wsl.Host)
	if meshHost == "" {
		return "", fmt.Errorf("wsl mesh host empty")
	}
	if p := u.Port(); p != "" {
		u.Host = fmt.Sprintf("%s:%s", meshHost, p)
	} else {
		u.Host = meshHost
	}
	return u.String(), nil
}

func RewriteLocalURLToWSLGuestIP(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u == nil {
		return "", nil
	}
	host := strings.TrimSpace(u.Hostname())
	if host != "127.0.0.1" && host != "localhost" {
		return raw, nil
	}
	ip := detectWSLGuestIP()
	if ip == "" {
		return RewriteLocalURLToWSLHost(raw)
	}
	if p := u.Port(); p != "" {
		u.Host = fmt.Sprintf("%s:%s", ip, p)
	} else {
		u.Host = ip
	}
	return u.String(), nil
}

func RewriteBrowserURLForRemoteNode(rawURL, remoteNode string) (string, error) {
	if !IsWSLRuntime() || strings.TrimSpace(remoteNode) == "" {
		return rawURL, nil
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(remoteNode))
	if err == nil && strings.EqualFold(strings.TrimSpace(node.OS), "windows") && node.PreferWSLPowerShell {
		if rewritten, rewriteErr := RewriteLocalURLToWSLGuestIP(rawURL); rewriteErr == nil && strings.TrimSpace(rewritten) != "" {
			return rewritten, nil
		}
	}
	if rewritten, err := RewriteLocalURLToWSLHost(rawURL); err == nil && strings.TrimSpace(rewritten) != "" {
		return rewritten, nil
	}
	return rawURL, nil
}

func detectWSLGuestIP() string {
	out, err := exec.Command("hostname", "-I").Output()
	if err != nil {
		return ""
	}
	for _, field := range strings.Fields(strings.TrimSpace(string(out))) {
		ip := strings.TrimSpace(field)
		if ip == "" || ip == "127.0.0.1" || strings.HasPrefix(ip, "100.") {
			continue
		}
		return ip
	}
	return ""
}
