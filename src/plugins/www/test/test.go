package test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"dialtone/cli/src/core/browser"
	"dialtone/cli/src/core/test"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func init() {
	test.Register("www-dev-server", "www", []string{"www", "integration", "browser"}, RunWwwIntegration)
	test.Register("www-cad", "www", []string{"www", "cad", "browser"}, RunWwwCadHeaded)
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
	browser.CleanupPort(8081) // CAD Proxy port
	
	// Use our new CLI for safe cleanup (defaults to Dialtone origin)
	_ = exec.Command("./dialtone.sh", "chrome", "kill", "all").Run()

	// 3. Start Dev Server (Background)
	fmt.Println(">> [WWW] Starting Dev Server on port 5173...")
	devCmd := exec.Command("npm", "run", "dev", "--", "--host", "127.0.0.1")
	devCmd.Dir = wwwDir
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

	// 5. Run ChromeDP Tests using chrome CLI and NewRemoteContext
	fmt.Println(">> [WWW] Launching Headed Chrome via CLI...")

	// Launch via CLI to get the signature and origin detection
	// Force GPU for faster rendering in tests
	args := []string{"chrome", "new", "--gpu"}
	
	launchCmd := exec.Command("./dialtone.sh", args...)
	output, err := launchCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to launch chrome via CLI: %v\nOutput: %s", err, string(output))
	}

	// Parse WebSocket URL from output
	// Format: WebSocket URL  : ws://127.0.0.1:XXXXX/devtools/browser/...
	re := regexp.MustCompile(`ws://127\.0\.0\.1:\d+/devtools/browser/[a-z0-9-]+`)
	wsURL := re.FindString(string(output))
	if wsURL == "" {
		return fmt.Errorf("failed to find WebSocket URL in CLI output: %s", string(output))
	}
	fmt.Printf(">> [WWW] Connected to Chrome via: %s\n", wsURL)

	// Attach to the browser started by the CLI
	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
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

	// Print console logs summary and fail if errors found
	if err := printConsoleLogs(consoleLogs); err != nil {
		return err
	}

	fmt.Println("\n[PASS] WWW Integration Tests Complete")

	// Final verification: No Dialtone processes leaked
	fmt.Println(">> [WWW] Verifying no leaked Dialtone processes...")
	_ = exec.Command("./dialtone.sh", "chrome", "kill", "all").Run()
	
	// Double check with list
	listCmd := exec.Command("./dialtone.sh", "chrome", "list")
	listOutput, _ := listCmd.CombinedOutput()
	if strings.Contains(string(listOutput), "Dialtone") {
		fmt.Printf(">> [WARNING] Leaked Dialtone processes detected:\n%s\n", string(listOutput))
	} else {
		fmt.Println(">> [WWW] Cleanup verified.")
	}

	return nil
}

// printConsoleLogs prints collected browser console logs and returns error if critical failures found
func printConsoleLogs(logs []string) error {
	fmt.Println("\n>> [WWW] Browser Console Logs:")
	fmt.Println("   ----------------------------------------")
	
	if len(logs) == 0 {
		fmt.Println("   (no console output)")
		return nil
	}

	var filtered []string
	for _, log := range logs {
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
	
	hasErrors := false
	for _, log := range logs {
		if strings.Contains(log, "[error]") || strings.Contains(log, "[EXCEPTION]") {
			hasErrors = true
			break
		}
	}
	
	fmt.Println("   ----------------------------------------")
	if hasErrors {
		fmt.Println("   [FAIL] Console errors or exceptions detected!")
		// Collect only errors/exceptions to show clearly
		fmt.Println("   CRITICAL ERRORS:")
		for _, log := range logs {
			if strings.Contains(log, "[error]") || strings.Contains(log, "[EXCEPTION]") {
				fmt.Printf("   >> %s\n", log)
			}
		}
		return fmt.Errorf("critical console errors detected during browser execution")
	} else {
		fmt.Printf("   [PASS] %d console messages, no errors\n", len(filtered))
	}
	return nil
}

// verifyHomePage checks the main landing page by navigating through sections
func verifyHomePage(ctx context.Context) error {
	fmt.Println(">> [WWW] Testing Home Page Sections...")

	// 1. Initial Page Load & Home Section
	var title string
	isLive := os.Getenv("CAD_LIVE") == "true"
	
	injectLiveFlag := chromedp.ActionFunc(func(ctx context.Context) error {
		if isLive {
			return chromedp.Evaluate(`window.CAD_LIVE = true`, nil).Do(ctx)
		}
		return nil
	})

	mockFetch := chromedp.ActionFunc(func(ctx context.Context) error {
		if isLive {
			fmt.Println(">> [WWW] Skipping CAD mock (CAD_LIVE=true)")
			return nil
		}
		return chromedp.Evaluate(`
			window.fetch = ((originalFetch) => {
				return async (...args) => {
					const url = args[0];
					if (url && url.includes("/api/cad/generate")) {
						console.log("[test] Mocking CAD POST /api/cad/generate");
						return {
							ok: true,
							arrayBuffer: async () => new ArrayBuffer(1024),
							blob: async () => new Blob([new ArrayBuffer(1024)])
						};
					}
					if (url && url.includes("/api/cad")) {
						console.log("[test] Mocking CAD API response (GET)");
						return {
							ok: true,
							json: async () => ({
								type: "gear",
								parameters: { num_teeth: 20, outer_diameter: 80 },
								source_code: "# mock source"
							})
						};
					}
					return originalFetch(...args);
				};
			})(window.fetch);
		`, nil).Do(ctx)
	})

	err := chromedp.Run(ctx,
		chromedp.Navigate("http://127.0.0.1:5173"),
		injectLiveFlag,
		mockFetch,
		chromedp.WaitReady("#earth-container"), // Wait for existence first
		chromedp.Title(&title),
	)
	if err != nil {
		return fmt.Errorf("initial load failed: %v", err)
	}
	if title != "dialtone.earth" {
		return fmt.Errorf("unexpected title: %s", title)
	}
	fmt.Println(">> [WWW] Initial page loaded, starting section verification...")

	sections := []struct {
		id       string
		headline string
		search   string
	}{
		{"s-home", "Now is the time to learn and build", "#earth-container"},
		{"s-robot", "Robotics begins with precision control", "#robot-container"},
		{"s-neural", "Mathematics powers autonomy", "#nn-container"},
		{"s-math", "Mathematics powers autonomy", "#math-container"},
		{"s-cad", "Parametric Design Logic", "#cad-container"},
		{"s-about", "Vision", "#about-container"},
		{"s-docs", "Documentation", "#docs-container"},
	}

	for i, s := range sections {
		fmt.Printf("   [%d/%d] Verifying section: %s\n", i+1, len(sections), s.id)
		
		var headline string
		var exists bool
		var isVisible bool

		actions := []chromedp.Action{
			chromedp.Evaluate(fmt.Sprintf(`!!document.getElementById("%s")`, s.id), &exists),
			chromedp.ActionFunc(func(ctx context.Context) error {
				// Try marketing overlay first, then h1 in page-content
				var query string
				if s.id == "s-about" || s.id == "s-docs" {
					query = fmt.Sprintf("#%s h1", s.id)
				} else {
					query = fmt.Sprintf("#%s .marketing-overlay h2", s.id)
				}
				return chromedp.Text(query, &headline, chromedp.ByQuery).Do(ctx)
			}),
			chromedp.Evaluate(fmt.Sprintf(`!!document.querySelector("%s")`, s.search), &isVisible),
		}

		if err := chromedp.Run(ctx, actions...); err != nil {
			return fmt.Errorf("verification failed for %s: %v", s.id, err)
		}

		if !exists {
			return fmt.Errorf("section %s not found", s.id)
		}
		if !strings.Contains(headline, s.headline) {
			return fmt.Errorf("unexpected headline for %s: %s (expected: %s)", s.id, headline, s.headline)
		}
		if !isVisible {
			return fmt.Errorf("container/element %s not found in %s", s.search, s.id)
		}

		// Navigate to next section using URL anchor
		if i < len(sections)-1 {
			nextId := sections[i+1].id
			fmt.Printf("   -> Navigating to next section using URL: #%s\n", nextId)
			if err := chromedp.Run(ctx, chromedp.Navigate(fmt.Sprintf("http://127.0.0.1:5173/#%s", nextId))); err != nil {
				return fmt.Errorf("failed to navigate to next section %s: %v", nextId, err)
			}
			// Wait for the next section element itself to have the is-visible class
			waitQuery := fmt.Sprintf("#%s.is-visible", nextId)
			if err := chromedp.Run(ctx, chromedp.WaitReady(waitQuery)); err != nil {
				return fmt.Errorf("timeout waiting for section %s to become active (.is-visible)", nextId)
			}
		}
	}

	fmt.Println("   [PASS] All Sections Verified")
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

// RunWwwCadHeaded starts a headed browser and scrolls to the CAD section
func RunWwwCadHeaded() error {
	fmt.Println(">> [WWW] Starting CAD Verification...")

	isLive := os.Getenv("CAD_LIVE") == "true"
	headless := os.Getenv("HEADLESS") == "true"

	// 1. Cleanup existing processes
	fmt.Println(">> [WWW] Cleaning up existing processes...")
	browser.CleanupPort(5173) // Dev server port
	if isLive {
		browser.CleanupPort(8081) // CAD port
	}
	
	// Use our new CLI for safe cleanup
	_ = exec.Command("./dialtone.sh", "chrome", "kill", "all").Run()
	time.Sleep(1 * time.Second)

	// 2. Start Dev Server (Background)
	cwd, _ := os.Getwd()
	wwwDir := filepath.Join(cwd, "src", "plugins", "www", "app")
	fmt.Println(">> [WWW] Starting Dev Server on port 5173...")
	devCmd := exec.Command("npm", "run", "dev", "--", "--host", "127.0.0.1")
	devCmd.Dir = wwwDir
	if err := devCmd.Start(); err != nil {
		return fmt.Errorf("failed to start dev server: %v", err)
	}
	defer func() {
		if devCmd.Process != nil {
			fmt.Println(">> [WWW] Stopping Dev Server...")
			devCmd.Process.Kill()
		}
	}()

	// 3. Start CAD Backend if Live requested
	if isLive {
		fmt.Println(">> [WWW] Starting CAD Backend on port 8081...")
		cadCmd := exec.Command("./dialtone.sh", "cad", "server")
		if err := cadCmd.Start(); err != nil {
			return fmt.Errorf("failed to start CAD backend: %v", err)
		}
		defer func() {
			if cadCmd.Process != nil {
				fmt.Println(">> [WWW] Stopping CAD Backend...")
				cadCmd.Process.Kill()
			}
		}()
		if err := waitForPort(8081, 15*time.Second); err != nil {
			return fmt.Errorf("CAD backend port 8081 not ready: %v", err)
		}
	}

	// 4. Wait for dev server
	if err := waitForPort(5173, 30*time.Second); err != nil {
		return fmt.Errorf("dev server port 5173 not ready: %v", err)
	}

	fmt.Println(">> [WWW] Launching Chrome via CLI...")
	
	args := []string{"chrome", "new", "--gpu"}
	if headless {
		args = append(args, "--headless")
	}

	launchCmd := exec.Command("./dialtone.sh", args...)
	output, err := launchCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to launch chrome via CLI: %v\nOutput: %s", err, string(output))
	}

	// Parse WebSocket URL
	re := regexp.MustCompile(`ws://127\.0\.0\.1:\d+/devtools/browser/[a-z0-9-]+`)
	wsURL := re.FindString(string(output))
	if wsURL == "" {
		return fmt.Errorf("failed to find WebSocket URL in CLI output: %s", string(output))
	}
	fmt.Printf(">> [WWW] Connected to Chrome via: %s\n", wsURL)

	// Attach
	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 300*time.Second) // Long timeout for manual viewing
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

	fmt.Printf(">> [WWW] Launching Chrome (Headless: %v) and navigating to CAD section...\n", headless)
	
	injectLiveFlag := chromedp.ActionFunc(func(ctx context.Context) error {
		if isLive {
			return chromedp.Evaluate(`window.CAD_LIVE = true`, nil).Do(ctx)
		}
		return nil
	})

	mockFetch := chromedp.ActionFunc(func(ctx context.Context) error {
		if isLive {
			fmt.Println(">> [WWW] Skipping CAD mock (CAD_LIVE=true)")
			return nil
		}
		return chromedp.Evaluate(`
			window.fetch = ((originalFetch) => {
				return async (...args) => {
					const url = args[0];
					if (url && url.includes("/api/cad/generate")) {
						console.log("[test] Mocking CAD POST /api/cad/generate");
						return {
							ok: true,
							arrayBuffer: async () => new ArrayBuffer(1024),
							blob: async () => new Blob([new ArrayBuffer(1024)])
						};
					}
					if (url && url.includes("/api/cad")) {
						console.log("[test] Mocking CAD API response (GET)");
						return {
							ok: true,
							json: async () => ({
								type: "gear",
								parameters: { num_teeth: 20, outer_diameter: 80 },
								source_code: "# mock source"
							})
						};
					}
					return originalFetch(...args);
				};
			})(window.fetch);
		`, nil).Do(ctx)
	})

	err = chromedp.Run(ctx,
		chromedp.Navigate("http://127.0.0.1:5173"),
		injectLiveFlag,
		mockFetch,
		chromedp.Sleep(2*time.Second),
		// Scroll to s-cad to trigger lazy load
		chromedp.Evaluate(`
			const scad = document.getElementById("s-cad");
			if (scad) {
				scad.scrollIntoView({ behavior: 'smooth' });
			}
		`, nil),
		chromedp.Sleep(20*time.Second), // Give it time to load live STL after scroll
		// Verify three.js gear loaded
		chromedp.ActionFunc(func(ctx context.Context) error {
			var loaded bool
			err := chromedp.Evaluate(`
				(function() {
					if (!window.cadViewer) return false;
					return window.cadViewer.gearGroup.children.length > 0;
				})()
			`, &loaded).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to check CAD status: %v", err)
			}
			if !loaded {
				return fmt.Errorf("CAD STL was not loaded into the Three.js scene")
			}
			fmt.Println("   [PASS] Verified CAD STL loaded in Three.js")
			return nil
		}),
		chromedp.Sleep(5*time.Second), // Briefly pause
	)

	if err != nil {
		printConsoleLogs(consoleLogs)
		return fmt.Errorf("CAD test failed: %v", err)
	}

	fmt.Println(">> [WWW] CAD Verification Complete.")
	
	// Final verification: No Dialtone processes leaked
	fmt.Println(">> [WWW] Verifying no leaked Dialtone processes...")
	_ = exec.Command("./dialtone.sh", "chrome", "kill", "all").Run()
	
	listCmd := exec.Command("./dialtone.sh", "chrome", "list")
	listOutput, _ := listCmd.CombinedOutput()
	if strings.Contains(string(listOutput), "Dialtone") {
		fmt.Printf(">> [WARNING] Leaked Dialtone processes detected:\n%s\n", string(listOutput))
	} else {
		fmt.Println(">> [WWW] Cleanup verified.")
	}

	// Final check for console errors
	return printConsoleLogs(consoleLogs)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
