package cli

import (
	"dialtone/cli/src/core/browser"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func RunDev(versionDir string) error {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "dag", versionDir)
	port := 8080

	fmt.Printf(">> [DAG] Dev: starting %s...\n", versionDir)

	uiDist := filepath.Join(pluginDir, "ui", "dist")
	if _, err := os.Stat(uiDist); os.IsNotExist(err) {
		fmt.Printf(">> [DAG] Dev: UI dist not found. Building first...\n")
		if err := RunBuild(versionDir); err != nil {
			return fmt.Errorf("failed to build UI: %v", err)
		}
	}

	browser.CleanupPort(port)

	cmd := exec.Command("go", "run", "cmd/main.go")
	cmd.Dir = pluginDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start dag host: %v", err)
	}

	go func() {
		for i := 0; i < 30; i++ {
			if browser.IsPortOpen(port) {
				fmt.Printf("\n🚀 DAG Plugin (%s) is READY!\n", versionDir)
				fmt.Printf("🔗 URL: http://localhost:%d\n\n", port)
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
		fmt.Printf("\n❌ [ERROR] Host node failed to start on port %d\n", port)
	}()

	fmt.Println(">> [DAG] Dev: host process started. Press Ctrl+C to stop.")
	return cmd.Wait()
}
