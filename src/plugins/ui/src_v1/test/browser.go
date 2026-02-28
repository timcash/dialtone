package test

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func BrowserOptionsFor(defaultURL string) (testv1.BrowserOptions, bool, error) {
	opts := GetOptions()
	attach := strings.TrimSpace(opts.AttachNode) != ""
	targetURL := strings.TrimSpace(opts.TargetURL)
	if targetURL == "" {
		targetURL = strings.TrimSpace(defaultURL)
	}

	b := testv1.BrowserOptions{
		Headless: true,
		GPU:      false,
		Role:     "ui-test",
		URL:      targetURL,
		// Keep a single attached tab and stable viewport for the full suite run.
		SkipNavigateOnReuse: true,
		PreserveTabAndSize:  true,
	}
	if !attach {
		// WSL local -> Windows local Chrome attach is unreliable in NAT mode.
		// Default to remote mesh browser unless explicitly opted out.
		if isWSL() && strings.TrimSpace(os.Getenv("DIALTONE_UI_TEST_FORCE_LOCAL")) == "" {
			defaultNode := strings.TrimSpace(os.Getenv("DIALTONE_UI_TEST_ATTACH_DEFAULT"))
			if defaultNode == "" {
				defaultNode = "darkmac"
			}
			if defaultNode != "" {
				rewritten := strings.TrimSpace(b.URL)
				if rewritten != "" {
					if r, err := rewriteLocalURLToWSLHost(rewritten); err == nil && strings.TrimSpace(r) != "" {
						rewritten = r
					}
				}
				b.Headless = true
				b.GPU = true
				b.ReuseExisting = true
				b.RemoteNode = defaultNode
				b.URL = rewritten
				return b, true, nil
			}
		}
		// Local test runs from WSL often launch Windows Chrome under the hood.
		// Rewrite localhost URLs to the mesh host so the browser can reach the served fixture.
		if isWSL() && strings.TrimSpace(b.URL) != "" {
			rewritten, err := rewriteLocalURLToWSLGuestIP(b.URL)
			if err != nil {
				return b, false, err
			}
			if rewritten != "" {
				b.URL = rewritten
			}
		}
		return b, false, nil
	}

	// Attach mode targets a headed browser session on the remote mesh node.
	b.Headless = false
	b.GPU = true
	// Reuse the long-lived test browser when attaching to remote nodes.
	b.Role = "test"
	b.ReuseExisting = true
	b.RemoteNode = strings.TrimSpace(opts.AttachNode)
	if strings.TrimSpace(b.URL) == "" {
		inferred, err := inferWSLURL(5177)
		if err != nil {
			return b, true, err
		}
		b.URL = inferred
		return b, true, nil
	}
	rewritten, err := rewriteLocalURLToWSLHost(b.URL)
	if err != nil {
		return b, true, err
	}
	if rewritten != "" {
		b.URL = rewritten
	}
	return b, true, nil
}

func isWSL() bool {
	raw, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(raw)), "microsoft")
}

func inferWSLURL(port int) (string, error) {
	wsl, err := sshv1.ResolveMeshNode("wsl")
	if err != nil {
		return "", err
	}
	host := strings.TrimSpace(wsl.Host)
	if host == "" {
		return "", fmt.Errorf("wsl mesh host empty")
	}
	return fmt.Sprintf("http://%s:%d", host, port), nil
}

func rewriteLocalURLToWSLHost(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u == nil {
		return "", nil
	}
	host := strings.TrimSpace(u.Hostname())
	if host != "127.0.0.1" && host != "localhost" {
		return "", nil
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

func rewriteLocalURLToWSLGuestIP(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u == nil {
		return "", nil
	}
	host := strings.TrimSpace(u.Hostname())
	if host != "127.0.0.1" && host != "localhost" {
		return "", nil
	}
	ip := detectWSLGuestIP()
	if ip == "" {
		return rewriteLocalURLToWSLHost(raw)
	}
	if p := u.Port(); p != "" {
		u.Host = fmt.Sprintf("%s:%s", ip, p)
	} else {
		u.Host = ip
	}
	return u.String(), nil
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
