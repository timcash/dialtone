package test

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

func Run07GoRun(ctx *testCtx) (string, error) {
	if err := ctx.ensureSharedServer(); err != nil {
		return "", err
	}

	var resp *http.Response
	var err error
	start := time.Now()
	for time.Since(start) < 5*time.Second {
		resp, err = http.Get("http://127.0.0.1:8080/health")
		if err == nil && resp.StatusCode == 200 {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}

	if err != nil {
		return "", fmt.Errorf("failed to reach /health: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("unexpected /health status: %d", resp.StatusCode)
	}

	return "Go server running.", nil
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
