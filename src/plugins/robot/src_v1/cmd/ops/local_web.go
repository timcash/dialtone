package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	test_v2 "dialtone/dev/plugins/test/src_v1/go"
)

func LocalWebRemoteRobot() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	repoRoot := cwd
	if filepath.Base(cwd) == "src" {
		repoRoot = filepath.Dir(cwd)
	}

	hostname := os.Getenv("DIALTONE_HOSTNAME")
	if hostname == "" {
		hostname = "drone-1"
	}

	fmt.Printf(">> [Robot] Starting Local UI connected to Remote Robot: %s\n", hostname)

	uiDir := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v1", "ui")

	// Set environment variable for Vite to use as proxy target
	// We'll proxy through Vite to the remote Tailscale IP/Hostname
	proxyTarget := fmt.Sprintf("http://%s:80", hostname)
	os.Setenv("VITE_PROXY_TARGET", proxyTarget)

	// 1. Start UI dev server
	devCmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "dev", "--host", "127.0.0.1", "--port", "3000")
	devCmd.Stdout = os.Stdout
	devCmd.Stderr = os.Stderr
	if err := devCmd.Start(); err != nil {
		return fmt.Errorf("failed to start dev server: %w", err)
	}
	defer devCmd.Process.Kill()

	// 2. Wait for dev server
	if err := test_v2.WaitForPort(3000, 15*time.Second); err != nil {
		return fmt.Errorf("dev server failed to start: %w", err)
	}

	// 3. Launch Chrome
	chromeCmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "chrome", "new", "http://127.0.0.1:3000", "--role", "dev", "--reuse-existing")
	chromeCmd.Stdout = os.Stdout
	chromeCmd.Stderr = os.Stderr
	if err := chromeCmd.Run(); err != nil {
		return fmt.Errorf("failed to launch chrome: %w", err)
	}

	// Keep alive
	fmt.Println(">> Local UI running. Press Ctrl+C to stop.")
	select {}
}
