package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"dialtone/cli/src/core/browser"
)

func Run18CleanupVerification() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	_ = browser.CleanupPort(8080)

	serve := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "template", "serve", "src_v3")
	serve.Dir = repoRoot
	serve.Stdout = os.Stdout
	serve.Stderr = os.Stderr
	if err := serve.Start(); err != nil {
		return err
	}

	if err := waitForPort("127.0.0.1:8080", 12*time.Second); err != nil {
		_ = serve.Process.Kill()
		_, _ = serve.Process.Wait()
		return err
	}

	_ = serve.Process.Kill()
	_, _ = serve.Process.Wait()
	_ = browser.CleanupPort(8080)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:8080", 300*time.Millisecond)
		if err != nil {
			return nil
		}
		_ = conn.Close()
		time.Sleep(150 * time.Millisecond)
	}

	return fmt.Errorf("cleanup verification failed: serve process still accepting connections on 8080")
}
