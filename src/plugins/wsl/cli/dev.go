package cli

import (
	"context"
	"dialtone/dev/browser"
	chrome_app "dialtone/dev/plugins/chrome/src_v1/go"
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
		buildCmd := exec.Command("bun", "run", "build")
		buildCmd.Dir = filepath.Join(pluginDir, "ui")
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("failed to build UI: %v", err)
		}
	}

	// 2. Cleanup port 8080
	browser.CleanupPort(port)

	// 3. Start the host node
	cmd := exec.Command("go", "run", "cmd/main.go")
	cmd.Dir = pluginDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start wsl host: %v", err)
	}

	// 4. Wait for it to be ready and launch browser
	go func() {
		for i := 0; i < 30; i++ {
			if browser.IsPortOpen(port) {
				fmt.Printf("\n🚀 WSL Plugin (%s) is READY!\n", versionDir)
				fmt.Printf("🔗 URL: http://localhost:%d\n\n", port)

				// Launch Debug Browser (Headed)
				launchDebugBrowser(port)
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
		fmt.Printf("\n❌ [ERROR] Host node failed to start on port %d\n", port)
	}()

	// 5. Block until interrupted
	fmt.Println(">> [WSL] Dev: host process started. Press Ctrl+C to stop.")
	return cmd.Wait()
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
