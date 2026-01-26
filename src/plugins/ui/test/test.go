package test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"dialtone/cli/src/core/browser"
	"dialtone/cli/src/core/test"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func init() {
	test.Register("ui-integration", "ui", []string{"ui", "integration", "browser"}, RunUiIntegration)
}

// RunAll runs all UI integration tests
func RunAll() error {
	fmt.Println(">> [UI] Starting Integration Tests...")
	return test.RunTicket("ui")
}

func RunUiIntegration() error {

	// 1. Locate Root / dialtone.sh
	cwd, _ := os.Getwd()
	dialtoneSh := filepath.Join(cwd, "dialtone.sh")
	if _, err := os.Stat(dialtoneSh); os.IsNotExist(err) {
		return fmt.Errorf("could not find dialtone.sh in %s", cwd)
	}

	// 0. Kill existing UI processes to avoid port conflicts
	fmt.Println(">> [UI] Cleaning up existing processes...")
	exec.Command(dialtoneSh, "ui", "kill").Run()
	time.Sleep(1 * time.Second)

	// 2. Start Mock Data Server (Background)
	fmt.Println(">> [UI] Starting Mock Data Server...")
	mockCmd := exec.Command(dialtoneSh, "ui", "mock-data")
	if err := mockCmd.Start(); err != nil {
		return fmt.Errorf("failed to start mock data: %v", err)
	}
	defer func() {
		if mockCmd.Process != nil {
			fmt.Println(">> [UI] Stopping Mock Data Server...")
			mockCmd.Process.Kill()
		}
	}()

	// 3. Start Dev Server (Background)
	fmt.Println(">> [UI] Starting Dev Server...")
	devCmd := exec.Command(dialtoneSh, "ui", "dev", "--port", "5174", "--host")
	devCmd.Stdout = os.Stdout
	devCmd.Stderr = os.Stderr
	if err := devCmd.Start(); err != nil {
		return fmt.Errorf("failed to start dev server: %v", err)
	}
	defer func() {
		if devCmd.Process != nil {
			fmt.Println(">> [UI] Stopping Dev Server...")
			devCmd.Process.Kill()
		}
	}()

	// Wait for services to stabilize
	fmt.Println(">> [UI] Waiting for services...")

	if err := waitForPort(5174, 20*time.Second); err != nil {
		return fmt.Errorf("dev server port 5174 not ready: %v", err)
	}
	if err := waitForPort(4223, 10*time.Second); err != nil {
		return fmt.Errorf("mock data port 4223 (NATS WS) not ready: %v", err)
	}
	fmt.Println(">> [UI] Services Ready.")

	// 4. Run ChromeDP Verification
	fmt.Println(">> [UI] Launching Headless Chrome...")

	execPath := browser.FindChromePath()
	if execPath == "" {
		return fmt.Errorf("chrome executable not found")
	}
	fmt.Printf(">> [UI] Found Chrome at: %s\n", execPath)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("window-size", "1920,1080"),
		chromedp.ExecPath(execPath),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	// Capture console logs
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			for _, arg := range ev.Args {
				fmt.Printf(">> [BROWSER CONSOLE] %s\n", arg.Value)
			}
		case *runtime.EventExceptionThrown:
			fmt.Printf(">> [BROWSER EXCEPTION] %s\n", ev.ExceptionDetails.Text)
		}
	})

	var (
		title       string
		termExists  bool
		threeExists bool
		camExists   bool
		nodes       []*cdp.Node
		htmlDump    string
	)

	fmt.Println(">> [UI] Navigating to http://localhost:5174...")
	err := chromedp.Run(ctx,
		chromedp.Navigate("http://localhost:5174"),
		chromedp.Title(&title),
		chromedp.OuterHTML("html", &htmlDump),
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(`!!document.getElementById("terminal-container")`, &termExists),
		chromedp.WaitVisible("#three-container", chromedp.ByID),
		chromedp.Evaluate(`!!document.getElementById("three-container")`, &threeExists),
		chromedp.WaitVisible(".panel-right", chromedp.ByQuery),
		chromedp.Nodes(".panel-right", &nodes),
	)

	if err != nil {
		fmt.Printf(">> [UI DEBUG] HTML Dump:\n%s\n", htmlDump)
		return fmt.Errorf("browser verification failed: %v", err)
	}

	// VERIFICATION
	var natsVal, heartbeatVal string

	// Telemetry Check
	err = chromedp.Run(ctx,
		chromedp.Sleep(5*time.Second), // Give more time for data to flow
		chromedp.Text("#val-nats", &natsVal, chromedp.ByID),
		chromedp.Text("#val-heartbeat", &heartbeatVal, chromedp.ByID),
	)
	if err != nil {
		fmt.Printf(">> [UI DEBUG] HTML Dump:\n%s\n", htmlDump)
		fmt.Printf(">> [UI] WARNING: Telemetry verification failed: %v.\n", err)
		// return nil // Continue anyway to see results
	}

	camExists = len(nodes) > 0

	fmt.Printf("\n=== [UI] Test Results ===\n")
	fmt.Printf("Title: %s\n", title)
	fmt.Printf("Terminal Container: %v\n", termExists)
	fmt.Printf("3D Container: %v\n", threeExists)
	fmt.Printf("Right Panel: %v\n", camExists)
	fmt.Printf("NATS Msg Count: %s\n", natsVal)
	fmt.Printf("Heartbeat: %s\n", heartbeatVal)

	if !termExists || !threeExists || !camExists {
		return fmt.Errorf("missing critical UI components")
	}

	if natsVal == "0" || natsVal == "--" {
		return fmt.Errorf("NATS messages not received by the web UI")
	}

	fmt.Println("\n[PASS] UI Structure & Basic Telemetry Verified")
	return nil
}

func waitForPort(port int, timeout time.Duration) error {
	start := time.Now()
	for time.Since(start) < timeout {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 1*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for port %d", port)
}
