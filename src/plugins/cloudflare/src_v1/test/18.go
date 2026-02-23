package main

import (
	"fmt"
	"net"
	"time"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
)

func Run18CleanupVerification() error {
	teardownSharedEnv()
	_ = chrome.CleanupPort(testServerPort)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", testServerPort), 300*time.Millisecond)
		if err != nil {
			return nil
		}
		_ = conn.Close()
		time.Sleep(150 * time.Millisecond)
	}

	return fmt.Errorf("cleanup verification failed: serve process still accepting connections on %d", testServerPort)
}
