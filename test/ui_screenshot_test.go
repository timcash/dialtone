package test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	dialtone "dialtone/cli/src"

	"github.com/chromedp/chromedp"
	"github.com/nats-io/nats-server/v2/server"
)

func TestUI_ScreenshotsAndMessaging(t *testing.T) {
	// 1. Setup local server
	natsPort := 16222
	wsPort := 16223
	webPort := 18081

	// Use a temp dir for state
	stateDir, _ := os.MkdirTemp("", "dialtone-ui-test-*")
	defer os.RemoveAll(stateDir)

	// Start NATS with WebSocket support
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
	handler := dialtone.CreateWebHandler("localhost", natsPort, wsPort, webPort, ns, nil, nil)

	srv := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", webPort),
		Handler: handler,
	}
	go srv.ListenAndServe()
	defer srv.Close()

	// Wait for web server
	time.Sleep(2 * time.Second)

	// 2. Setup Chromedp
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Working directory when running 'go test ./test' is 'test'
	screenshotDir := "screenshots"
	if _, err := os.Stat(screenshotDir); os.IsNotExist(err) {
		err := os.MkdirAll(screenshotDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create screenshot dir: %v", err)
		}
	}

	var buf []byte
	targetURL := fmt.Sprintf("http://127.0.0.1:%d", webPort)

	err = chromedp.Run(ctx,
		chromedp.Navigate(targetURL),
		chromedp.WaitVisible(`[aria-label="System Status"].status-online`, chromedp.ByQuery),

		// Take initial screenshot
		chromedp.CaptureScreenshot(&buf),
		chromedp.ActionFunc(func(ctx context.Context) error {
			path := filepath.Join(screenshotDir, "initial_load.png")
			dialtone.LogPrintf("Saving screenshot to %s", path)
			return os.WriteFile(path, buf, 0644)
		}),

		// Fill inputs
		chromedp.SendKeys(`[aria-label="NATS Subject"]`, "test.ui.message", chromedp.ByQuery),
		chromedp.SendKeys(`[aria-label="Message Body"]`, `{"hello": "world"}`, chromedp.ByQuery),

		// Take screenshot before send
		chromedp.CaptureScreenshot(&buf),
		chromedp.ActionFunc(func(ctx context.Context) error {
			path := filepath.Join(screenshotDir, "before_send.png")
			return os.WriteFile(path, buf, 0644)
		}),

		// Click send
		chromedp.Click(`[aria-label="Send NATS Message"]`, chromedp.ByQuery),

		// Wait for log entry
		chromedp.WaitVisible(`[aria-label="Log Entry success"]`, chromedp.ByQuery),

		// Take final screenshot
		chromedp.CaptureScreenshot(&buf),
		chromedp.ActionFunc(func(ctx context.Context) error {
			path := filepath.Join(screenshotDir, "after_send.png")
			return os.WriteFile(path, buf, 0644)
		}),
	)

	if err != nil {
		t.Fatalf("UI Test failed: %v", err)
	}

	dialtone.LogInfo("UI screenshots saved successfully")
}
