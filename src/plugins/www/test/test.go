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
	browser.CleanupPort(8081) // CAD Proxy port
	
	// Aggressively kill chrome instances as requested
	browser.KillProcessesByName("chrome")
	browser.KillProcessesByName("google-chrome")
	browser.KillProcessesByName("chromium")
	
	time.Sleep(1 * time.Second)

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

	// 5. Run ChromeDP Tests using chrome plugin patterns
	fmt.Println(">> [WWW] Launching Headless Chrome...")

	execPath := browser.FindChromePath()
	if execPath == "" {
		return fmt.Errorf("chrome executable not found")
	}
	fmt.Printf(">> [WWW] Found Chrome at: %s\n", execPath)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.ExecPath(execPath),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("window-size", "1920,1080"),
		chromedp.Flag("remote-debugging-address", "127.0.0.1"),
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
	} else {
		fmt.Printf("   [PASS] %d console messages, no errors\n", len(filtered))
	}
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
		chromedp.Sleep(2*time.Second),
		chromedp.Title(&title),
	)
	if err != nil {
		return fmt.Errorf("initial load failed: %v", err)
	}
	if title != "dialtone.earth" {
		return fmt.Errorf("unexpected title: %s", title)
	}

	sections := []struct {
		id       string
		headline string
		search   string
	}{
		{"s-home", "Now is the time to learn and build", "#earth-container"},
		{"s-robot", "Robotics begins with precision control", "#robot-container"},
		{"s-video", "Communication networks make robots real-time", ".bg-video"},
		{"s-neural", "Mathematics powers autonomy", "#nn-container"},
		{"s-curriculum", "Build the future, step by step", "#curriculum-container"},
		{"s-cad", "Parametric Design Logic", "#cad-container"},
	}

	for i, s := range sections {
		fmt.Printf("   [%d/%d] Verifying section: %s\n", i+1, len(sections), s.id)
		
		var headline string
		var exists bool
		var isVisible bool

		actions := []chromedp.Action{
			chromedp.Evaluate(fmt.Sprintf(`!!document.getElementById("%s")`, s.id), &exists),
			chromedp.Text(fmt.Sprintf("#%s .marketing-overlay h2", s.id), &headline, chromedp.ByQuery),
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

		// Navigate to next section using simulated ArrowDown
		if i < len(sections)-1 {
			fmt.Printf("   -> Navigating to next section using ArrowDown\n")
			if err := chromedp.Run(ctx, chromedp.KeyEvent("\u21e3")); err != nil { // ArrowDown
				return fmt.Errorf("failed to navigate to next section: %v", err)
			}
			time.Sleep(1 * time.Second) // Wait for snap animation
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
	fmt.Println(">> [WWW] Starting Headed CAD Verification...")

	// 1. Cleanup existing processes
	fmt.Println(">> [WWW] Cleaning up existing processes...")
	browser.CleanupPort(5173) // Dev server port
	browser.KillProcessesByName("chrome")
	browser.KillProcessesByName("google-chrome")
	browser.KillProcessesByName("chromium")
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

	// 3. Wait for dev server
	if err := waitForPort(5173, 30*time.Second); err != nil {
		return fmt.Errorf("dev server port 5173 not ready: %v", err)
	}

	execPath := browser.FindChromePath()
	if execPath == "" {
		return fmt.Errorf("chrome executable not found")
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.ExecPath(execPath),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("window-size", "1280,720"),
		// Force headed mode
		chromedp.Flag("headless", false),
		chromedp.Flag("remote-debugging-address", "127.0.0.1"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
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

	fmt.Println(">> [WWW] Launching Headed Chrome and navigating to CAD section...")
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
		chromedp.Sleep(2*time.Second),
		// Scroll to s-cad to trigger lazy load
		chromedp.Evaluate(`
			const scad = document.getElementById("s-cad");
			if (scad) {
				scad.scrollIntoView({ behavior: 'smooth' });
			}
		`, nil),
		chromedp.Sleep(10*time.Second), // Give it time to load live STL after scroll
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
				printConsoleLogs(consoleLogs)
				return fmt.Errorf("CAD STL was not loaded into the Three.js scene")
			}
			fmt.Println("   [PASS] Verified CAD STL loaded in Three.js")
			return nil
		}),
		chromedp.Sleep(10*time.Second), // Pause for user inspection
	)

	if err != nil {
		return fmt.Errorf("headed CAD test failed: %v", err)
	}

	fmt.Println(">> [WWW] Headed CAD Verification Complete.")
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
