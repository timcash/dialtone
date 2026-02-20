package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	test_v2 "dialtone/dev/plugins/test"
)

func Run10DevServerRunningLatestUI() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	port, err := test_v2.PickFreePort()
	if err != nil {
		return err
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	url := fmt.Sprintf("http://%s", addr)

	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "vpn", "ui-run", "src_v1", "--port", fmt.Sprintf("%d", port))
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

	if err := test_v2.WaitForPort(port, 12*time.Second); err != nil {
		return err
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected dev server status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	html := string(body)
	if !strings.Contains(html, "Hero Section") {
		return fmt.Errorf("dev server did not serve expected latest UI content")
	}

	return nil
}
