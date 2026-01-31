package test

import (
	"context"
	"dialtone/cli/src/core/browser"
	"dialtone/cli/src/dialtest"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func init() {
	dialtest.RegisterTicket("www-robot-text")
	dialtest.AddSubtaskTest("init", RunInitTest, nil)
	dialtest.AddSubtaskTest("change-branch", RunChangeBranchTest, nil)
	dialtest.AddSubtaskTest("add-marketing-text", func() error { 
		return checkBrowser("Unified Networks marketing information", "Global Coordination") 
	}, nil)
	dialtest.AddSubtaskTest("add-robot-kit-offer", checkBrowserStripe, nil)
	dialtest.AddSubtaskTest("verify-browser", FinalBrowserPass, nil)
}

func RunInitTest() error { return nil }

func RunChangeBranchTest() error {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil { return err }
	branch := strings.TrimSpace(string(out))
	if branch != "www-robot-text" {
		return fmt.Errorf("expected branch www-robot-text, got %s", branch)
	}
	return nil
}

func FinalBrowserPass() error {
	return checkBrowser("Precision Control marketing information", "Inverse kinematics")
}

func checkBrowser(ariaLabel, textInside string) error {
	ctx, cancel, _ := setupBrowser()
	defer cancel()

	var labelFound, textFound bool
	err := chromedp.Run(ctx,
		chromedp.Navigate("http://127.0.0.1:5173"),
		chromedp.Sleep(2*time.Second),
		// Press Space to find the right section
		chromedp.KeyEvent(" "), 
		chromedp.Sleep(1*time.Second),
		chromedp.KeyEvent(" "),
		chromedp.Sleep(1*time.Second),
		// Use aria-label selector
		chromedp.Evaluate(fmt.Sprintf(`!!document.querySelector('[aria-label="%s"]')`, ariaLabel), &labelFound),
		chromedp.Evaluate(fmt.Sprintf(`document.body.textContent.includes("%s")`, textInside), &textFound),
	)
	if err != nil { return err }

	if !labelFound { return fmt.Errorf("element with aria-label '%s' not found", ariaLabel) }
	if !textFound { return fmt.Errorf("text '%s' not found in page body", textInside) }
	return nil
}

func checkBrowserStripe() error {
	ctx, cancel, _ := setupBrowser()
	defer cancel()

	var labelFound, priceFound bool
	err := chromedp.Run(ctx,
		chromedp.Navigate("http://127.0.0.1:5173"),
		chromedp.Sleep(2*time.Second),
		// Cycle to the end (approx 5 spaces for 6 slides)
		chromedp.KeyEvent(" "), chromedp.Sleep(500*time.Millisecond),
		chromedp.KeyEvent(" "), chromedp.Sleep(500*time.Millisecond),
		chromedp.KeyEvent(" "), chromedp.Sleep(500*time.Millisecond),
		chromedp.KeyEvent(" "), chromedp.Sleep(500*time.Millisecond),
		chromedp.KeyEvent(" "), chromedp.Sleep(500*time.Millisecond),
		
		chromedp.Evaluate(`!!document.querySelector('[aria-label="Order section: Robot Kit offer"]')`, &labelFound),
		chromedp.Evaluate(`document.body.textContent.includes("1,000")`, &priceFound),
	)
	if err != nil { return err }

	if !labelFound { return fmt.Errorf("Stripe section aria-label not found") }
	if !priceFound { return fmt.Errorf("price 1,000 not found in page body") }
	return nil
}

// setupBrowser efficiently reuses existing Chrome debugger or starts ONE headless instance.
func setupBrowser() (context.Context, context.CancelFunc, error) {
	// 1. Ensure Dev Server is running
	if !isPortOpen(5173) {
		cwd, _ := os.Getwd()
		devCmd := exec.Command("npm", "run", "dev", "--", "--host", "127.0.0.1")
		devCmd.Dir = filepath.Join(cwd, "src", "plugins", "www", "app")
		devCmd.Start()
		waitForPort(5173, 10*time.Second)
	}

	// 2. Check if a Chrome debugger is ALREADY running on 9222
	// If it is, we ATTACH instead of spawning a new one.
	if isPortOpen(9222) {
		// Found existing debugger (likely from previous test run or user)
		allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), "http://127.0.0.1:9222")
		ctx, cancelCtx := chromedp.NewContext(allocCtx)
		return ctx, func() {
			cancelCtx()
			cancel()
		}, nil
	}

	// 3. Otherwise, spawn a fresh HEADLESS chrome and KEEP it around 
	// for the duration of this GO process (or use remote debugging pattern)
	execPath := browser.FindChromePath()
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Headless,
		chromedp.ExecPath(execPath),
		chromedp.Flag("remote-debugging-port", "9222"),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	
	return ctx, func() {
		cancelCtx()
		cancel()
	}, nil
}

func isPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	return false
}

func waitForPort(port int, timeout time.Duration) error {
	start := time.Now()
	for time.Since(start) < timeout {
		if isPortOpen(port) { return nil }
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for port %d", port)
}
