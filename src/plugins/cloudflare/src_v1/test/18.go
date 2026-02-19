package main

import (
	"fmt"
	"net"
	"time"

	"dialtone/dev/core/browser"
)

func Run18CleanupVerification() error {
	teardownSharedEnv()
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
