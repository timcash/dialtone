package test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dialtone/cli/src/core/browser"
	"dialtone/cli/src/core/test"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func init() {
	test.Register("www-dev-server", "www", []string{"www", "integration", "browser"}, RunWwwIntegration)
}

// RunAll runs all www integration tests - entry point for `./dialtone.sh plugin test www`
func RunAll() error {
	fmt.Println(">> [WWW] Starting Integration Tests...")
	return RunWwwIntegration()
}

// RunWwwIntegration starts the dev server and runs chromedp tests
func RunWwwIntegration() error {
	// 1. Locate Root / dialtone.sh
	cwd, _ := os.Getwd()
	dialtoneSh := filepath.Join(cwd, "dialtone.sh")
	if _, err := os.Stat(dialtoneSh); os.IsNotExist(err) {
		return fmt.Errorf("could not find dialtone.sh in %s", cwd)
	}

	wwwDir := filepath.Join(cwd, "src", "plugins", "www", "app")
	if _, err := os.Stat(wwwDir); os.IsNotExist(err) {
		return fmt.Errorf("www app directory not found: %s", wwwDir)
	}

	// 2. Cleanup existing processes using browser utilities
	fmt.Println(">> [WWW] Cleaning up existing processes...")
	browser.CleanupPort(5173) // Dev server port
	time.Sleep(500 * time.Millisecond)

	// 3. Start Dev Server (Background)
	fmt.Println(">> [WWW] Starting Dev Server on port 5173...")
	devCmd := exec.Command("npm", "run", "dev", "--", "--host", "127.0.0.1")
	devCmd.Dir = wwwDir
	// Uncomment for debugging:
	// devCmd.Stdout = os.Stdout
	// devCmd.Stderr = os.Stderr
	if err := devCmd.Start(); err != nil {
		return fmt.Errorf("failed to start dev server: %v", err)
	}
	defer func() {
		if devCmd.Process != nil {
			fmt.Println(">> [WWW] Stopping Dev Server...")
			devCmd.Process.Kill()
		}
	}()

	// 4. Wait for dev server to be ready
	fmt.Println(">> [WWW] Waiting for dev server...")
	if err := waitForPort(5173, 30*time.Second); err != nil {
		return fmt.Errorf("dev server port 5173 not ready: %v", err)
	}
	fmt.Println(">> [WWW] Dev Server Ready.")

	// 5. Run ChromeDP Tests using chrome plugin patterns
	fmt.Println(">> [WWW] Launching Headless Chrome...")

	execPath := browser.FindChromePath()
	if execPath == "" {
		return fmt.Errorf("chrome executable not found - run './dialtone.sh chrome' to verify Chrome installation")
	}
	fmt.Printf(">> [WWW] Found Chrome at: %s\n", execPath)

	// Use chrome plugin's recommended options for WSL compatibility
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.ExecPath(execPath),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("window-size", "1920,1080"),
		chromedp.Flag("remote-debugging-address", "127.0.0.1"), // Force IPv4 for WSL
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Collect console.log events from the browser
	var consoleLogs []string
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			for _, arg := range ev.Args {
				consoleLogs = append(consoleLogs, fmt.Sprintf("[%s] %s", ev.Type, arg.Value))
			}
		case *runtime.EventExceptionThrown:
			consoleLogs = append(consoleLogs, fmt.Sprintf("[EXCEPTION] %s", ev.ExceptionDetails.Text))
		}
	})

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Run verification tests
	if err := verifyHomePage(ctx); err != nil {
		return err
	}

	// Print console logs summary
	printConsoleLogs(consoleLogs)

	fmt.Println("\n[PASS] WWW Integration Tests Complete")
	return nil
}

// printConsoleLogs prints collected browser console logs
func printConsoleLogs(logs []string) {
	fmt.Println("\n>> [WWW] Browser Console Logs:")
	fmt.Println("   ----------------------------------------")
	
	if len(logs) == 0 {
		fmt.Println("   (no console output)")
		return
	}

	// Filter out CSS style strings (the %c arguments)
	var filtered []string
	for _, log := range logs {
		// Skip CSS color/style strings
		if strings.HasPrefix(log, "[log] \"color:") || 
		   strings.HasPrefix(log, "[debug] \"color:") ||
		   strings.Contains(log, "font-weight:") ||
		   strings.Contains(log, "font-size:") {
			continue
		}
		filtered = append(filtered, log)
	}

	for _, log := range filtered {
		fmt.Printf("   %s\n", log)
	}
	
	// Check for errors/exceptions
	hasErrors := false
	for _, log := range logs {
		if strings.Contains(log, "[error]") || strings.Contains(log, "[EXCEPTION]") {
			hasErrors = true
			break
		}
	}
	
	fmt.Println("   ----------------------------------------")
	if hasErrors {
		fmt.Println("   [WARN] Console errors detected!")
	} else {
		fmt.Printf("   [PASS] %d console messages, no errors\n", len(filtered))
	}
}

// verifyHomePage checks the main landing page
func verifyHomePage(ctx context.Context) error {
	fmt.Println(">> [WWW] Testing Home Page...")

	var (
		title                   string
		heroHeadline            string
		heroButtonHref          string
		heroButtonText          string
		robotHeadline           string
		videoHeadline           string
		neuralHeadline          string
		curriculumHeadline      string
		earthExists             bool
		robotExists             bool
		nnExists                bool
		heroCentered            bool
		heroHeadlineWhite       bool
		stripeSectionExists     bool
		htmlDump                string
	)

	err := chromedp.Run(ctx,
		chromedp.Navigate("http://127.0.0.1:5173"),
		chromedp.Sleep(2*time.Second), // Wait for page to render
		chromedp.Title(&title),
		chromedp.OuterHTML("html", &htmlDump),
		chromedp.Text("#s-home .marketing-overlay h2", &heroHeadline, chromedp.ByQuery),
		chromedp.Text("#s-home .marketing-overlay .buy-button", &heroButtonText, chromedp.ByQuery),
		chromedp.AttributeValue("#s-home .marketing-overlay .buy-button", "href", &heroButtonHref, nil, chromedp.ByQuery),
		chromedp.Text("#s-robot .marketing-overlay h2", &robotHeadline, chromedp.ByQuery),
		chromedp.Text("#s-video .marketing-overlay h2", &videoHeadline, chromedp.ByQuery),
		chromedp.Text("#s-neural .marketing-overlay h2", &neuralHeadline, chromedp.ByQuery),
		chromedp.Text("#s-curriculum .marketing-overlay h2", &curriculumHeadline, chromedp.ByQuery),
		chromedp.Evaluate(`!!document.getElementById("earth-container")`, &earthExists),
		chromedp.Evaluate(`!!document.getElementById("robot-container")`, &robotExists),
		chromedp.Evaluate(`!!document.getElementById("nn-container")`, &nnExists),
		chromedp.Evaluate(`!!document.getElementById("s-stripe")`, &stripeSectionExists),
		chromedp.Evaluate(`getComputedStyle(document.querySelector("#s-home .marketing-overlay")).textAlign === "center"`, &heroCentered),
		chromedp.Evaluate(`
			(() => {
				const h = document.querySelector("#s-home .marketing-overlay h2");
				if (!h) return false;
				return getComputedStyle(h).color === "rgb(255, 255, 255)";
			})()
		`, &heroHeadlineWhite),
	)

	if err != nil {
		fmt.Printf(">> [WWW DEBUG] HTML Dump:\n%s\n", htmlDump[:min(len(htmlDump), 2000)])
		return fmt.Errorf("home page verification failed: %v", err)
	}

	fmt.Printf("   Title: %s\n", title)
	fmt.Printf("   Hero Headline: %s\n", heroHeadline)
	fmt.Printf("   Hero Button: %s (%s)\n", heroButtonText, heroButtonHref)
	fmt.Printf("   Robot Headline: %s\n", robotHeadline)
	fmt.Printf("   Video Headline: %s\n", videoHeadline)
	fmt.Printf("   Neural Headline: %s\n", neuralHeadline)
	fmt.Printf("   Curriculum Headline: %s\n", curriculumHeadline)
	fmt.Printf("   Earth Container: %v\n", earthExists)
	fmt.Printf("   Robot Container: %v\n", robotExists)
	fmt.Printf("   Neural Network Container: %v\n", nnExists)
	fmt.Printf("   Hero Centered: %v\n", heroCentered)
	fmt.Printf("   Hero Headline White: %v\n", heroHeadlineWhite)

	if title != "dialtone.earth" {
		return fmt.Errorf("unexpected title: %s (expected: dialtone.earth)", title)
	}

	if !earthExists {
		return fmt.Errorf("earth-container not found")
	}

	if !robotExists {
		return fmt.Errorf("robot-container not found")
	}

	if !nnExists {
		return fmt.Errorf("nn-container not found")
	}

	if heroHeadline != "Now is the time to learn and build" {
		return fmt.Errorf("unexpected hero headline: %s", heroHeadline)
	}

	if heroButtonText != "Get the Robot Kit" {
		return fmt.Errorf("unexpected hero button text: %s", heroButtonText)
	}

	if !strings.Contains(heroButtonHref, "buy.stripe.com") {
		return fmt.Errorf("unexpected hero button href: %s", heroButtonHref)
	}

	if robotHeadline == "" || videoHeadline == "" || neuralHeadline == "" || curriculumHeadline == "" {
		return fmt.Errorf("missing marketing headlines")
	}

	if stripeSectionExists {
		return fmt.Errorf("stripe section should be removed")
	}

	if !heroCentered {
		return fmt.Errorf("hero marketing overlay is not centered")
	}

	if !heroHeadlineWhite {
		return fmt.Errorf("hero headline is not white")
	}

	fmt.Println("   [PASS] Home Page Verified")
	return nil
}

// waitForPort waits for a TCP port to become available
func waitForPort(port int, timeout time.Duration) error {
	start := time.Now()
	for time.Since(start) < timeout {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for port %d", port)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
