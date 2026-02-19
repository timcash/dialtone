package test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	chrome "dialtone/dev/plugins/chrome/app"

	"github.com/chromedp/chromedp"
)


func RunComprehensiveSmoke() error {
	fmt.Println(">> [WWW] Comprehensive Smoke Suite: start")
	cwd, _ := os.Getwd()
	wwwDir := filepath.Join(cwd, "src", "plugins", "www", "app")

	// 0. Cleanup BEFORE start using plugin tool
	fmt.Println(">> [WWW] Pre-test browser cleanup...")
	chrome.KillDialtoneResources()

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

	// 2. Start Managed Session via Chrome Plugin
	fmt.Println(">> [WWW] Launching single managed Chrome session...")
	session, err := chrome.StartSession(chrome.SessionOptions{
		Role:          "smoke",
		Headless:      true,
		GPU:           true,
		ReuseExisting: false,
	})
	if err != nil {
		return fmt.Errorf("failed to start chrome session: %v", err)
	}
	defer func() {
		fmt.Println(">> [WWW] Cleaning up managed Chrome session...")
		chrome.CleanupSession(session)
	}()

	// 3. Attach chromedp to the session
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), session.WebSocketURL)
	defer allocCancel()

	// Create ONE shared tab context
	tabCtx, tabCancel := chromedp.NewContext(allocCtx)
	defer tabCancel()

	// 4. Run Section Smoke Test
	fmt.Println("\n>> [WWW] Running Section Performance & Stability Test...")
	if err := RunWwwSmokeSubTest(tabCtx); err != nil {
		return fmt.Errorf("performance smoke test failed: %v", err)
	}

	// 5. Run Menu Smoke Test
	fmt.Println("\n>> [WWW] Running Menu Lifecycle & Interaction Test...")
	if err := RunWwwMenuSmokeSubTest(tabCtx); err != nil {
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
