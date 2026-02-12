package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"dialtone/cli/src/core/browser"
)

func Run07GoRun() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	_ = browser.CleanupPort(8080)

	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "template", "serve", "src_v3")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	if err := waitForPort("127.0.0.1:8080", 12*time.Second); err != nil {
		return err
	}

	resp, err := http.Get("http://127.0.0.1:8080/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected /health status: %d", resp.StatusCode)
	}

	return nil
}

func waitForPort(addr string, timeout time.Duration) error {
	start := time.Now()
	for time.Since(start) < timeout {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", addr)
}
