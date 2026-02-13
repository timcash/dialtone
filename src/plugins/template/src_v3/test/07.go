package main

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

func Run07GoRun() error {
	if err := ensureSharedServer(); err != nil {
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
