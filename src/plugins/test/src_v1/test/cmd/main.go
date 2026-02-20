package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	fmt.Println("Running test plugin suite (src_v1)...")

	broker, err := logs.StartEmbeddedNATS()
	if err != nil {
		fmt.Printf("FAIL: embedded nats start failed: %v\n", err)
		os.Exit(1)
	}
	defer broker.Close()

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}
	logPath := filepath.Join(repoRoot, "src", "plugins", "test", "src_v1", "test", "test.log")
	_ = os.Remove(logPath)

	subject := "logs.test.src_v1"
	stop, err := logs.ListenToFile(broker.Conn(), subject, logPath)
	if err != nil {
		fmt.Printf("FAIL: listen to file failed: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = stop() }()

	logger, err := logs.NewNATSLogger(broker.Conn(), subject)
	if err != nil {
		fmt.Printf("FAIL: new nats logger failed: %v\n", err)
		os.Exit(1)
	}

	if err := logger.Infof("test plugin info message"); err != nil {
		fmt.Printf("FAIL: info publish failed: %v\n", err)
		os.Exit(1)
	}
	if err := logger.Errorf("test plugin error message"); err != nil {
		fmt.Printf("FAIL: error publish failed: %v\n", err)
		os.Exit(1)
	}

	if err := waitForContains(logPath, "test plugin info message", 4*time.Second); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}
	if err := waitForContains(logPath, "test plugin error message", 4*time.Second); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("PASS: logs library integration verified (subject=%s, file=%s)\n", subject, logPath)
}

func waitForContains(path, pattern string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), pattern) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %q in %s", pattern, path)
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("repo root not found")
		}
		cwd = parent
	}
}
