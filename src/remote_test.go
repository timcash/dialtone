package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/nats-io/nats-server/v2/server"
)

// isResolvable checks if a hostname can be resolved to an IP address.
func isResolvable(host string) bool {
	ips, err := net.LookupIP(host)
	return err == nil && len(ips) > 0
}

// =============================================================================
// Remote System Tests (verifies live rover at 192.168.4.36 or via Tailscale)
// =============================================================================

func TestRemoteRoverWebUI(t *testing.T) {
	targetHost := os.Getenv("ROVER_HOST")
	if targetHost == "" {
		targetHost = "dialtone" // Use the Tailscale node name
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

	// Capture console logs (very useful for debugging remote JS issues)
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
		// Take a screenshot on failure to see what's wrong
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

// TestNatsWebUIConnection verifies that the integrated web UI successfully
// connects to NATS via WebSocket. (Moved from dialtone_test.go)
func TestNatsWebUIConnection(t *testing.T) {
	// 1. Setup local server
	hostname := "localhost"
	natsPort := 15222
	wsPort := 15223
	webPort := 18080

	// Use a temp dir for state
	stateDir, _ := os.MkdirTemp("", "dialtone-test-*")
	defer os.RemoveAll(stateDir)

	// Start NATS
	opts := &server.Options{
		Host: "127.0.0.1",
		Port: natsPort,
		Websocket: server.WebsocketOpts{
			Port:  wsPort,
			NoTLS: true,
		},
	}
	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("Failed to create NATS: %v", err)
	}
	go ns.Start()
	defer ns.Shutdown()

	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatal("NATS not ready")
	}

	// Create web handler
	// We pass nil for LocalClient as we're testing locally without Tailscale context
	handler := createWebHandler(hostname, natsPort, wsPort, webPort, ns, nil, nil)

	server := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", webPort),
		Handler: handler,
	}
	go server.ListenAndServe()
	defer server.Close()

	// Wait for web server
	time.Sleep(2 * time.Second)

	// Check if web server is up
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", webPort))
	if err != nil {
		t.Fatalf("Web server not reachable: %v", err)
	}
	resp.Body.Close()
	t.Log("Web server is reachable")

	// Check if NATS WS is up
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", wsPort), 2*time.Second)
	if err != nil {
		t.Fatalf("NATS WS port not reachable: %v", err)
	}
	conn.Close()
	t.Log("NATS WS port is reachable")

	// 2. Automated Browser Test
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Capture console logs
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			for _, arg := range ev.Args {
				t.Logf("BROWSER LOG: %s", arg.Value)
			}
		}
	})

	// Set a timeout for the browser actions
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var statusText string
	err = chromedp.Run(ctx,
		chromedp.Navigate(fmt.Sprintf("http://127.0.0.1:%d", webPort)),
		// Wait for the status text to contain "CONNECTED"
		chromedp.WaitVisible(`#status-indicator.status-online`, chromedp.ByQuery),
		chromedp.TextContent(`#status-text`, &statusText, chromedp.ByID),
	)

	if err != nil {
		// Take a screenshot on failure
		var buf []byte
		if err2 := chromedp.Run(ctx, chromedp.FullScreenshot(&buf, 100)); err2 == nil {
			os.WriteFile("failure_status.png", buf, 0644)
			t.Log("Saved failure_status.png")
		}
		t.Fatalf("chromedp error: %v", err)
	}

	t.Logf("Final UI Status: %s", statusText)
	if !strings.Contains(statusText, "CONNECTED") {
		t.Errorf("Expected CONNECTED status, got %q", statusText)

		// Take a screenshot on failure
		var buf []byte
		if err := chromedp.Run(ctx, chromedp.Screenshot("#status-text", &buf, chromedp.ByID)); err == nil {
			os.WriteFile("failure_status.png", buf, 0644)
			t.Log("Saved failure_status.png")
		}
	}
}
