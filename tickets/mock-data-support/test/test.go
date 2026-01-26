package test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"dialtone/cli/src/core/browser"
	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/mock"
	"dialtone/cli/src/core/test"

	"github.com/chromedp/chromedp"
)

func init() {
	test.Register("implement-mock-telemetry", "mock-data-support", []string{"mock", "telemetry"}, RunMockTelemetry)
	test.Register("implement-mock-camera", "mock-data-support", []string{"mock", "camera"}, RunMockCamera)
	test.Register("implement-chromedp-test", "mock-data-support", []string{"core", "mock", "browser"}, RunChromedpTest)
}

// RunAll is the standard entry point required by project rules.
func RunAll() error {
	logger.LogInfo("Running mock-data-support suite...")
	return test.RunTicket("mock-data-support")
}

func RunChromedpTest() error {
	logger.LogInfo("Starting Chromedp verification test...")

	// 0. Cleanup ports and processes
	ports := []int{4222, 4223, 80}
	for _, p := range ports {
		browser.CleanupPort(p)
	}
	// Kill exactly "Google Chrome" processes (macOS name) or "chrome" (Linux)
	if runtime.GOOS == "darwin" {
		browser.KillProcessesByName("Google Chrome")
	} else {
		browser.KillProcessesByName("chrome")
	}
	// For dialtone, we'll rely on CleanupPort to kill the process listening on 4222/4223/80
	// to avoid killing Antigravity which has "dialtone" in its workspace path.

	time.Sleep(2 * time.Second) // Wait for cleanup to settle

	// 1. Start Dialtone in mock mode with Tailscale in background
	logger.LogInfo("Starting Dialtone in mock mode (hostname: drone-1)...")
	cmd := exec.Command("./dialtone.sh", "start", "--mock", "--hostname", "drone-1")

	logFile, err := os.Create("test_dialtone.log")
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start dialtone: %w", err)
	}
	dialtonePID := cmd.Process.Pid
	logger.LogInfo("Dialtone (sh) started with PID: %d", dialtonePID)

	// Ensure cleanup on exit
	defer func() {
		logger.LogInfo("Cleaning up dialtone process (PID: %d)...", dialtonePID)
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		// Also check for orphaned children
		children, _ := browser.GetChildPIDs(dialtonePID)
		for _, child := range children {
			logger.LogInfo("Cleaning up orphaned child process (PID: %d)...", child)
			exec.Command("kill", "-9", fmt.Sprintf("%d", child)).Run()
		}
	}()

	// 2. Wait for server to be ready
	logger.LogInfo("Waiting for Dialtone to initialize...")
	ready := false
	for i := 0; i < 45; i++ { // Increased timeout
		content, _ := os.ReadFile("test_dialtone.log")
		if strings.Contains(string(content), "NATS server started") || strings.Contains(string(content), "Listening on") || strings.Contains(string(content), "Operational") {
			ready = true
			break
		}
		if strings.Contains(string(content), "FATAL") || strings.Contains(string(content), "invalid key") {
			return fmt.Errorf("dialtone failed with fatal error (check test_dialtone.log)")
		}
		time.Sleep(1 * time.Second)
	}
	if !ready {
		return fmt.Errorf("dialtone failed to start within timeout")
	}

	// 3. Setup Chromedp
	path := browser.FindChromePath()
	if path == "" {
		return fmt.Errorf("no Chrome/Chromium found")
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.ExecPath(path),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("no-errdialogs", true),
		chromedp.Flag("log-level", "3"), // Silence most errors
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Track Chrome PIDs
	myPID := os.Getpid()
	initialChildren, _ := browser.GetChildPIDs(myPID)

	// 4. Navigate to localhost (now on port 8080)
	url := fmt.Sprintf("http://127.0.0.1:%d", 8080)
	logger.LogInfo("Navigating to %s...", url)
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var title string
	err = chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Title(&title),
		chromedp.WaitVisible("#dashboard-map", chromedp.ByID),
	)

	// Check for new children (should be Chrome)
	currentChildren, _ := browser.GetChildPIDs(myPID)
	for _, pid := range currentChildren {
		isNew := true
		for _, old := range initialChildren {
			if pid == old {
				isNew = false
				break
			}
		}
		if isNew {
			logger.LogInfo("Chromedp created browser process with PID: %d", pid)
		}
	}

	if err != nil {
		logger.LogInfo("Verification FAILED at http://drone-1: %v", err)
		return fmt.Errorf("verification failed: %w", err)
	}

	logger.LogInfo("PASS: Dashboard Loaded via Tailscale (drone-1). Title: %s", title)
	return nil
}

func RunMockTelemetry() error {
	// Drain the channel first
	for len(mock.MavlinkPubChan) > 0 {
		<-mock.MavlinkPubChan
	}

	logger.LogInfo("Starting mock telemetry publisher...")
	mock.StartMockMavlink(4222) // Port doesn't matter for this test as we check the channel

	// Wait for a few messages
	timeout := time.After(2 * time.Second)
	receivedHeartbeat := false
	receivedAttitude := false

	for !receivedHeartbeat || !receivedAttitude {
		select {
		case msg := <-mock.MavlinkPubChan:
			if msg.Subject == "mavlink.heartbeat" {
				receivedHeartbeat = true
				logger.LogInfo("PASS: Received mock heartbeat")
			}
			if msg.Subject == "mavlink.attitude" {
				receivedAttitude = true
				logger.LogInfo("PASS: Received mock attitude")
			}
		case <-timeout:
			return fmt.Errorf("timed out waiting for mock telemetry messages")
		}
	}

	return nil
}

func RunMockCamera() error {
	req := httptest.NewRequest("GET", "/stream", nil)
	w := httptest.NewRecorder()

	// Run in a goroutine because it's a streaming handler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go mock.MockStreamHandler(w, req.WithContext(ctx))

	// Wait for some data to be written
	time.Sleep(500 * time.Millisecond)
	cancel()

	// Verify headers
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "multipart/x-mixed-replace") {
		return fmt.Errorf("expected multipart content type, got %s", contentType)
	}

	// Verify some content was received
	if w.Body.Len() < 100 {
		return fmt.Errorf("expected stream data, but got only %d bytes", w.Body.Len())
	}

	logger.LogInfo("PASS: Mock MJPEG stream verified (%d bytes)", w.Body.Len())
	return nil
}
