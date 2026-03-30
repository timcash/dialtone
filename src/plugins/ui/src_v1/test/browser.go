package test

import (
	"fmt"
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
		// Local test runs from WSL often launch Windows Chrome under the hood.
		// Rewrite localhost URLs to the mesh host so the browser can reach the served fixture.
		if testv1.IsWSLRuntime() && strings.TrimSpace(b.URL) != "" {
			rewritten, err := testv1.RewriteLocalURLToWSLGuestIP(b.URL)
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
	if meshNode, err := sshv1.ResolveMeshNode(strings.TrimSpace(opts.AttachNode)); err == nil && strings.EqualFold(strings.TrimSpace(meshNode.OS), "windows") && meshNode.PreferWSLPowerShell {
		// WSL localhost forwarding is available on the paired Windows host; keep the
		// local URL so the headed Chrome session opens the dev server reliably.
		return b, true, nil
	}
	rewritten, err := testv1.RewriteLocalURLToWSLHost(b.URL)
	if err != nil {
		return b, true, err
	}
	if rewritten != "" {
		b.URL = rewritten
	}
	return b, true, nil
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
