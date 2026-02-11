package cli

import (
	"context"
	"dialtone/cli/src/core/browser"
	chrome_app "dialtone/cli/src/plugins/chrome/app"
	"fmt"
	cdruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func RunDev(versionDir string) error {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "wsl", versionDir)
	port := 8080

	fmt.Printf(">> [WSL] Dev: starting %s...\n", versionDir)

	// 1. Check if UI is built
	uiDist := filepath.Join(pluginDir, "ui", "dist")
	if _, err := os.Stat(uiDist); os.IsNotExist(err) {
		fmt.Printf(">> [WSL] Dev: UI dist not found. Building first...\n")
		uiDir := filepath.Join(pluginDir, "ui")
		var buildCmd *exec.Cmd
		if os.Getenv("OS") == "Windows_NT" {
			buildCmd = exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(cwd, "dialtone.ps1"), "bun", "exec", "--cwd", uiDir, "run", "build")
		} else {
			buildCmd = exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "build")
		}
		buildCmd.Dir = cwd
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("failed to build UI: %v", err)
		}
	}

	// 2. Cleanup port 8080 and dev port 3000
	browser.CleanupPort(port)
	browser.CleanupPort(3000)

	// 3. Start the host node
	backendCmd := exec.Command("go", "run", "cmd/main.go")
	backendCmd.Dir = pluginDir
	backendCmd.Stdout = os.Stdout
	backendCmd.Stderr = os.Stderr

	if err := backendCmd.Start(); err != nil {
		return fmt.Errorf("failed to start wsl host: %v", err)
	}

	// 4. Start the UI dev server
	uiDir := filepath.Join(pluginDir, "ui")
	var uiCmd *exec.Cmd
	if os.Getenv("OS") == "Windows_NT" {
		uiCmd = exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(cwd, "dialtone.ps1"), "bun", "exec", "--cwd", uiDir, "run", "dev", "--port", "3000")
	} else {
		uiCmd = exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "dev", "--port", "3000")
	}
	uiCmd.Dir = cwd
	uiCmd.Stdout = os.Stdout
	uiCmd.Stderr = os.Stderr

	if err := uiCmd.Start(); err != nil {
		backendCmd.Process.Kill()
		return fmt.Errorf("failed to start UI dev server: %v", err)
	}

	// 5. Wait for it to be ready and launch browser
	go func() {
		for i := 0; i < 30; i++ {
			if browser.IsPortOpen(3000) {
				fmt.Printf("\n🚀 WSL Plugin (%s) UI is READY!\n", versionDir)
				fmt.Printf("🔗 URL: http://localhost:3000\n\n")

				// Launch Debug Browser (Headed)
				launchDebugBrowser(3000)
				return
			}
			time.Sleep(1 * time.Second)
		}
		fmt.Printf("\n❌ [ERROR] UI dev server failed to start on port 3000\n")
	}()

	// 6. Block until interrupted
	fmt.Println(">> [WSL] Dev: processes started. Press Ctrl+C to stop.")
	
	// Wait for either to exit
	go func() {
		uiCmd.Wait()
		backendCmd.Process.Kill()
	}()
	
	return backendCmd.Wait()
}

func launchDebugBrowser(port int) {
	fmt.Println(">> [WSL] Dev: Launching debug browser (HEADED)...")
	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	res, err := chrome_app.LaunchChrome(0, true, false, url) // false = headed
	if err != nil {
		fmt.Printf("   [ERROR] Failed to launch browser: %v\n", err)
		return
	}

	allocCtx, _ := chromedp.NewRemoteAllocator(context.Background(), res.WebsocketURL)
	ctx, _ := chromedp.NewContext(allocCtx)

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *cdruntime.EventConsoleAPICalled:
			fmt.Printf("   [BROWSER] [%s] %v\n", ev.Type, ev.Args)
		case *cdruntime.EventExceptionThrown:
			fmt.Printf("   [BROWSER] [ERROR] %s\n", ev.ExceptionDetails.Text)
		}
	})

	if err := chromedp.Run(ctx); err != nil {
		fmt.Printf("   [BROWSER] Connection closed: %v\n", err)
	}
}
