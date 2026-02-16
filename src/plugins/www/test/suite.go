package test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"dialtone/cli/src/core/test"
	"dialtone/cli/src/core/browser"

	"github.com/chromedp/chromedp"
)

func init() {
	test.Register("www-smoke-suite", "www", []string{"www", "smoke", "browser", "comprehensive"}, RunComprehensiveSmoke)
}

func RunComprehensiveSmoke() error {
	fmt.Println(">> [WWW] Comprehensive Smoke Suite: start")
	cwd, _ := os.Getwd()
	wwwDir := filepath.Join(cwd, "src", "plugins", "www", "app")

	// 1. Ensure Servers are up
	if !isPortOpenSuite(4173) {
		fmt.Println(">> [WWW] Starting Preview Server on port 4173...")
		devCmd := exec.Command("npm", "run", "preview", "--", "--host", "127.0.0.1")
		devCmd.Dir = wwwDir; devCmd.Start(); defer devCmd.Process.Kill()
	}
	if !isPortOpenSuite(8081) {
		fmt.Println(">> [WWW] Starting CAD Server on port 8081...")
		cadCmd := exec.Command("./dialtone.sh", "cad", "server")
		cadCmd.Dir = cwd; cadCmd.Start(); defer cadCmd.Process.Kill()
	}
	waitForPortLocalSuite(4173, 60*time.Second)
	waitForPortLocalSuite(8081, 60*time.Second)

	// 2. Single Shared Browser Allocator
	chromePath := browser.FindChromePath()
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun, chromedp.NoDefaultBrowserCheck,
		chromedp.ExecPath(chromePath),
		chromedp.Flag("enable-precise-memory-info", true),
		chromedp.Headless,
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	// 3. Run Section Smoke Test
	fmt.Println("\n>> [WWW] Running Section Performance & Stability Test...")
	if err := RunWwwSmokeSubTest(allocCtx); err != nil {
		return fmt.Errorf("performance smoke test failed: %v", err)
	}

	// 4. Run Menu Smoke Test
	fmt.Println("\n>> [WWW] Running Menu Lifecycle & Interaction Test...")
	if err := RunWwwMenuSmokeSubTest(allocCtx); err != nil {
		return fmt.Errorf("menu smoke test failed: %v", err)
	}

	fmt.Println("\n>> [WWW] Comprehensive Smoke Suite: pass")
	return nil
}

func waitForPortLocalSuite(port int, timeout time.Duration) {
	start := time.Now()
	for time.Since(start) < timeout {
		if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second); err == nil {
			conn.Close(); return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func isPortOpenSuite(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err == nil { conn.Close(); return true }
	return false
}
