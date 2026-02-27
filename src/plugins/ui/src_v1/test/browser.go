package test

import (
	"fmt"
	"os"
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
	}
	if !attach {
		return b, false, nil
	}

	b.Headless = false
	b.GPU = true
	b.Role = "ui-dev"
	b.ReuseExisting = true
	b.RemoteNode = strings.TrimSpace(opts.AttachNode)
	if strings.TrimSpace(b.URL) == "" {
		inferred, err := inferWSLURL(5177)
		if err != nil {
			return b, true, err
		}
		b.URL = inferred
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
