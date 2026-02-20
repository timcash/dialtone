package cli

import (
	"dialtone/dev/browser"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func RunDev(versionDir string) error {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "nix", versionDir)
	port := 8080

	fmt.Printf(">> [NIX] Dev: starting %s...\n", versionDir)

	// 1. Check if UI is built
	uiDist := filepath.Join(pluginDir, "ui", "dist")
	if _, err := os.Stat(uiDist); os.IsNotExist(err) {
		fmt.Printf(">> [NIX] Dev: UI dist not found. Building first...\n")
		// We use bun from nix shell to build
		nixPath := "/nix/var/nix/profiles/default/bin"
		currentPath := os.Getenv("PATH")
		fullPath := nixPath + ":" + currentPath
		nixCmd := "export PATH=\"" + fullPath + "\"; export NIX_REMOTE=daemon; nix --extra-experimental-features \"nix-command flakes\" shell nixpkgs#bun -c bun run build"

		buildCmd := exec.Command("bash", "-c", nixCmd)
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
		return fmt.Errorf("failed to start nix host: %v", err)
	}

	// 4. Wait for it to be ready
	go func() {
		for i := 0; i < 30; i++ {
			if browser.IsPortOpen(port) {
				fmt.Printf("\n🚀 Nix Plugin (%s) is READY!\n", versionDir)
				fmt.Printf("🔗 URL: http://localhost:%d\n\n", port)
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
		fmt.Printf("\n❌ [ERROR] Host node failed to start on port %d\n", port)
	}()

	// 5. Block until interrupted
	fmt.Println(">> [NIX] Dev: host process started. Press Ctrl+C to stop.")
	return cmd.Wait()
}
