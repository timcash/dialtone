package test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestE2E_LocalDev(t *testing.T) {
	// 1. Locate the app directory
	wd, _ := os.Getwd()
	// tickets/www-simple-html/test -> ../../../src/plugins/www/app
	appDir := filepath.Join(wd, "../../../src/plugins/www/app")
	
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		t.Fatalf("App directory not found at: %s", appDir)
	}

	// 2. Ensure dependencies are installed (optional for test speed, but good for correctness)
	// We assume 'npm install' will be run as part of the implementation or manually before testing?
	// For robust E2E, we might want to run it.
	// t.Log("Running npm install...")
	// installCmd := exec.Command("npm", "install")
	// installCmd.Dir = appDir
	// if out, err := installCmd.CombinedOutput(); err != nil {
	// 	t.Fatalf("npm install failed: %v\n%s", err, out)
	// }

	// 3. Start Dev Server
	port := "5173" // Default Vite port
	t.Logf("Starting dev server on port %s...", port)
	
	// We use 'npm run dev' which we expect to map to 'vite'
	cmd := exec.Command("npm", "run", "dev", "--", "--port", port) 
	cmd.Dir = appDir
	
	// Create a pipe to capture output if needed, but for now just start it
	// We put it in a process group to kill it later
	// SysProcAttr is OS specific, skipping for simplicity in this snippet 
	// but purely killing cmd.Process should verify.
	
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start dev server: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	// 4. Poll for readiness
	baseURL := fmt.Sprintf("http://localhost:%s", port)
	ready := false
	var resp *http.Response
	var err error

	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err = http.Get(baseURL)
		if err == nil && resp.StatusCode == 200 {
			ready = true
			break
		}
	}

	if !ready {
		t.Fatalf("Server failed to allow connection at %s after 10s. Last error: %v", baseURL, err)
	}
	defer resp.Body.Close()

	// 5. Verify Content using Chromedp (configured for WSL/Remote)
	// We use RemoteAllocator to connect to Windows Chrome via port 9222
	// as per documentation for WSL environments where local Chrome is unavailable.
	allocatorCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), "ws://127.0.0.1:9222/")
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocatorCtx)
	defer cancel()

	// Set timeout
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second) // Increased timeout for remote connection
	defer cancel()

	var bodyString string
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL),
		chromedp.WaitVisible(`#app`, chromedp.ByQuery),
		chromedp.OuterHTML(`body`, &bodyString, chromedp.ByQuery),
	)
	if err != nil {
		t.Logf("Warning: Remote Chrome verification failed: %v", err)
		t.Log("Ensure Chrome is running on Windows with: start chrome --remote-debugging-port=9222")
		// We fail the test to signal it's required, or we could soft-fail if it's considered optional.
		// Given user requirement 'use chromedp', we should fail if it doesn't work.
		t.FailNow()
	}

	// Verify Content
	if !strings.Contains(bodyString, "dialtone.earth") {
		t.Errorf("Expected 'dialtone.earth' in body, got: %s", bodyString)
	}

	// Check for Navigation Links (Home, About, Docs)
	if !strings.Contains(bodyString, `aria-label="Home"`) {
		t.Errorf("Expected Home link not found")
	}
	if !strings.Contains(bodyString, `aria-label="About"`) {
		t.Errorf("Expected About link not found")
	}
	if !strings.Contains(bodyString, `aria-label="Docs"`) {
		t.Errorf("Expected Docs link not found")
	}

	// Check for GitHub Link
	if !strings.Contains(bodyString, `aria-label="GitHub"`) {
		t.Errorf("Expected GitHub link not found")
	}

	// 6. Verify Navigation to About Page
	var aboutBody string
	err = chromedp.Run(ctx,
		chromedp.Click(`a[aria-label="About"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`.page-content`, chromedp.ByQuery),
		chromedp.OuterHTML(`body`, &aboutBody, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("Failed to navigate to About page: %v", err)
	}

	if !strings.Contains(aboutBody, "Vision") { // "Vision" is the H1 on About page
		t.Errorf("Expected 'Vision' on About page")
	}

	t.Log("E2E Test Passed: Server is running, serving content, and navigation verified via Chromedp.")
}
