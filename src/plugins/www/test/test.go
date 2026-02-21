package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// RunAll runs all www integration tests
func RunAll() error {
	fmt.Println(">> [WWW] Starting Integration Tests...")
	return RunWwwIntegration()
}

// RunWwwIntegration starts the dev server and runs chromedp tests
func RunWwwIntegration() error {
	cwd, _ := os.Getwd()
	wwwDir := filepath.Join(cwd, "src", "plugins", "www", "app")

	fmt.Println(">> [WWW] Cleaning up existing processes...")
	_ = chrome.KillDialtoneResources()

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

	if err := waitForPort(5173, 30*time.Second); err != nil {
		return fmt.Errorf("dev server port 5173 not ready: %v", err)
	}

	session, err := chrome.StartSession(chrome.SessionOptions{
		Role: "integration", Headless: true, GPU: true,
	})
	if err != nil {
		return err
	}
	defer chrome.CleanupSession(session)

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), session.WebSocketURL)
	defer allocCancel()

	ctx, tabCancel := chromedp.NewContext(allocCtx)
	defer tabCancel()

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if err := verifyHomePage(ctx); err != nil {
		return err
	}

	fmt.Println("\n[PASS] WWW Integration Tests Complete")
	return nil
}

func RunStandaloneSmoke(name string) error {
	session, err := chrome.StartSession(chrome.SessionOptions{
		Role: "smoke", Headless: true, GPU: true,
	})
	if err != nil {
		return err
	}
	defer chrome.CleanupSession(session)

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), session.WebSocketURL)
	defer allocCancel()

	tabCtx, tabCancel := chromedp.NewContext(allocCtx)
	defer tabCancel()

	if name == "www-smoke" {
		return RunWwwSmokeSubTest(tabCtx)
	}
	return RunWwwMenuSmokeSubTest(tabCtx)
}

func formatRemoteObject(o *runtime.RemoteObject) string {
	if o == nil {
		return ""
	}
	if len(o.Value) > 0 {
		var v interface{}
		if err := json.Unmarshal(o.Value, &v); err == nil {
			b, _ := json.Marshal(v)
			return string(b)
		}
		return string(o.Value)
	}
	if o.UnserializableValue != "" {
		return string(o.UnserializableValue)
	}
	if o.Description != "" {
		return o.Description
	}
	if o.Type != "" {
		return string(o.Type)
	}
	return ""
}

func verifyHomePage(ctx context.Context) error {
	fmt.Println(">> [WWW] Testing Home Page Sections...")
	err := chromedp.Run(ctx,
		chromedp.Navigate("http://127.0.0.1:5173"),
		chromedp.WaitReady("#earth-container"),
	)
	if err != nil {
		return fmt.Errorf("initial load failed: %v", err)
	}

	sections := []string{"s-home", "s-robot", "s-neural", "s-math", "s-cad", "s-about", "s-policy"}
	for i, s := range sections {
		fmt.Printf("   [%d/%d] Verifying section: %s\n", i+1, len(sections), s)
		err := chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf("window.location.hash='%s'", s), nil),
			chromedp.WaitVisible(fmt.Sprintf("#%s.is-ready", s)),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

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

func RunWwwCadHeaded() error {
	_ = chrome.KillDialtoneResources()
	cwd, _ := os.Getwd()
	wwwDir := filepath.Join(cwd, "src", "plugins", "www", "app")
	devCmd := exec.Command("npm", "run", "dev", "--", "--host", "127.0.0.1")
	devCmd.Dir = wwwDir
	devCmd.Start()
	defer devCmd.Process.Kill()

	cadCmd := exec.Command("./dialtone.sh", "cad", "server")
	cadCmd.Start()
	defer cadCmd.Process.Kill()

	waitForPort(5173, 30*time.Second)
	waitForPort(8081, 15*time.Second)

	session, err := chrome.StartSession(chrome.SessionOptions{
		Role: "cad-test", Headless: false, GPU: true, TargetURL: "http://127.0.0.1:5173/#s-cad",
	})
	if err != nil {
		return err
	}
	defer chrome.CleanupSession(session)

	fmt.Println(">> [WWW] CAD Headed session active. Press Ctrl+C to close.")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	return nil
}

func RunSmokeChromeLifecycleTest() error {
	fmt.Println(">> [WWW] Chrome Lifecycle Test: start")

	initialProcs, _ := chrome.ListResources(true)
	dialtoneInitial := 0
	for _, p := range initialProcs {
		if p.Origin == "Dialtone" {
			dialtoneInitial++
		}
	}
	fmt.Printf(">> [WWW] Initial Dialtone Chrome processes: %d (Total: %d)\n", dialtoneInitial, len(initialProcs))

	fmt.Println(">> [WWW] Starting test browser lifecycle...")
	if err := performMinimalLifecycle(); err != nil {
		return fmt.Errorf("lifecycle failed: %v", err)
	}

	fmt.Println(">> [WWW] Waiting 5s for process cleanup...")
	time.Sleep(5 * time.Second)

	finalProcs, _ := chrome.ListResources(true)
	dialtoneFinal := 0
	for _, p := range finalProcs {
		if p.Origin == "Dialtone" {
			dialtoneFinal++
		}
	}
	fmt.Printf(">> [WWW] Final Dialtone Chrome processes: %d (Total: %d)\n", dialtoneFinal, len(finalProcs))

	if dialtoneFinal > dialtoneInitial {
		return fmt.Errorf("LEAK DETECTED: %d Dialtone Chrome processes remaining (started with %d)", dialtoneFinal, dialtoneInitial)
	}

	fmt.Println(">> [WWW] Chrome Lifecycle Test: pass (No leaks detected)")
	return nil
}

func performMinimalLifecycle() error {
	session, err := chrome.StartSession(chrome.SessionOptions{
		Role: "lifecycle-test", Headless: true, GPU: true,
	})
	if err != nil {
		return err
	}

	defer func() {
		fmt.Println("   [DEBUG] Calling chrome.CleanupSession...")
		chrome.CleanupSession(session)
	}()

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), session.WebSocketURL)
	defer allocCancel()

	ctx, tabCancel := chromedp.NewContext(allocCtx)
	defer tabCancel()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	fmt.Println("   [DEBUG] Navigating to about:blank...")
	return chromedp.Run(ctx, chromedp.Navigate("about:blank"))
}
