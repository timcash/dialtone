package test

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// isResolvable checks if a hostname can be resolved to an IP address.
func isResolvable(host string) bool {
	ips, err := net.LookupIP(host)
	return err == nil && len(ips) > 0
}

// =============================================================================
// Remote System Tests (verifies live rover via Tailscale)
// =============================================================================

func TestRemoteRover_WebUI(t *testing.T) {
	targetHost := os.Getenv("ROVER_HOST")
	if targetHost == "" {
		targetHost = "dialtone-1" // Use the default dialtone-1 Tailscale node name
	}

	// Skip if host is not resolvable
	if !isResolvable(targetHost) {
		t.Skipf("Skipping remote test: host %q not resolvable. Is Tailscale connected and %q node up?", targetHost, targetHost)
	}

	targetURL := fmt.Sprintf("http://%s", targetHost)
	t.Logf("Checking remote rover UI at %s", targetURL)

	// 1. Setup Chromedp
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Capture console logs
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			for _, arg := range ev.Args {
				t.Logf("browser console: %s", arg.Value)
			}
		}
	})

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var statusText string
	err := chromedp.Run(ctx,
		chromedp.Navigate(targetURL),
		// Wait for the status indicator to reflect an online NATS connection
		chromedp.WaitVisible(`#status-indicator.status-online`, chromedp.ByQuery),
		chromedp.TextContent(`#status-text`, &statusText, chromedp.ByID),
	)

	if err != nil {
		// Take a screenshot on failure
		var buf []byte
		if err2 := chromedp.Run(ctx, chromedp.FullScreenshot(&buf, 100)); err2 == nil {
			os.WriteFile("remote_failure.png", buf, 0644)
			t.Log("Saved remote_failure.png")
		}
		t.Fatalf("Remote UI check failed for %s: %v", targetURL, err)
	}

	t.Logf("Remote UI Status: %s", statusText)
	if !strings.Contains(statusText, "CONNECTED") {
		t.Errorf("Expected status 'CONNECTED', got %q", statusText)
	}
}
