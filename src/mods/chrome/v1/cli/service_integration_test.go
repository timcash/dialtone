package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

var chromeV1Integration = flag.Bool("chrome-v1-integration", false, "run chrome/v1 integration tests")

func TestServiceStartAndStatus(t *testing.T) {
	runServiceStartAndHealthCheck(t, true)
}

func TestServiceStartModes(t *testing.T) {
	t.Run("new-headed", func(t *testing.T) {
		runServiceStartAndHealthCheck(t, false)
	})
	t.Run("new-headless", func(t *testing.T) {
		runServiceStartAndHealthCheck(t, true)
	})
	t.Run("tab-flow", func(t *testing.T) {
		runServiceTabFlow(t)
	})
	t.Run("headed-tab-count", func(t *testing.T) {
		runServiceHeadedTabCount(t)
	})
}

func runServiceHeadedTabCount(t *testing.T) {
	t.Helper()
	requireChromeV1Integration(t)

	repoRoot, err := locateRepoRoot()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	pidPath := chromeServicePIDPath(repoRoot)
	if running, pid := serviceRunning(pidPath); running {
		t.Logf("chrome v1 service already running (pid=%d), stopping before test", pid)
		if err := runStop(nil); err != nil {
			t.Fatalf("stop existing service: %v", err)
		}
	}

	host := "127.0.0.1"
	httpPort := mustFreePort(t)
	natsPort := mustFreePort(t)
	chromeDebugPort := mustFreePort(t)
	natsURL := fmt.Sprintf("nats://127.0.0.1:%d", natsPort)
	natsPrefix := "chrome.v1.test"

	startArgs := []string{
		"--host", host,
		"--port", strconv.Itoa(httpPort),
		"--nats-url", natsURL,
		"--nats-prefix", natsPrefix,
		"--embedded-nats",
		"--chrome-debug-port", strconv.Itoa(chromeDebugPort),
		"--initial-url", "https://www.google.com",
		// NO --headless here
	}
	if err := runStart(startArgs); err != nil {
		t.Fatalf("runStart: %v", err)
	}
	defer func() {
		if err := runStop(nil); err != nil {
			t.Logf("runStop cleanup: %v", err)
		}
	}()

	healthURL := fmt.Sprintf("http://%s:%d/healthz", host, httpPort)
	if err := waitForHealthz(healthURL, 10*time.Second); err != nil {
		t.Fatalf("service health check failed: %v", err)
	}

	baseOpts := natsControlOptions{
		natsURL:    natsURL,
		natsPrefix: natsPrefix,
		timeout:    20 * time.Second,
	}

	resp, err := requestNATSCommand(baseOpts, ".tab.list", controlRequest{})
	if err != nil {
		t.Fatalf("list initial tabs: %v", err)
	}
	if len(resp.Tabs) != 1 {
		t.Errorf("expected EXACTLY 1 tab in headed mode, got %d: %+v", len(resp.Tabs), resp.Tabs)
	}
}

func mustFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate free port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func waitForHealthz(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:gosec
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK && strings.TrimSpace(string(body)) == "ok" {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s", url)
}

func runServiceStartAndHealthCheck(t *testing.T, headless bool) {
	t.Helper()
	requireChromeV1Integration(t)

	repoRoot, err := locateRepoRoot()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	pidPath := chromeServicePIDPath(repoRoot)
	if running, pid := serviceRunning(pidPath); running {
		t.Logf("chrome v1 service already running (pid=%d), stopping before test", pid)
		if err := runStop(nil); err != nil {
			t.Fatalf("stop existing service: %v", err)
		}
	}

	host := "127.0.0.1"
	httpPort := mustFreePort(t)
	natsPort := mustFreePort(t)
	chromeDebugPort := mustFreePort(t)

	startArgs := []string{
		"--host", host,
		"--port", strconv.Itoa(httpPort),
		"--nats-url", fmt.Sprintf("nats://127.0.0.1:%d", natsPort),
		"--nats-prefix", "chrome.v1.test",
		"--embedded-nats",
		"--chrome-debug-port", strconv.Itoa(chromeDebugPort),
		"--initial-url", "https://www.google.com",
	}
	if headless {
		startArgs = append(startArgs, "--headless")
	}

	if err := runStart(startArgs); err != nil {
		t.Fatalf("runStart: %v", err)
	}
	defer func() {
		if err := runStop(nil); err != nil {
			t.Logf("runStop cleanup: %v", err)
		}
	}()

	if ok, _ := serviceRunning(pidPath); !ok {
		t.Fatalf("expected serviceRunning=true after start")
	}

	healthURL := fmt.Sprintf("http://%s:%d/healthz", host, httpPort)
	if err := waitForHealthz(healthURL, 10*time.Second); err != nil {
		t.Fatalf("service health check failed: %v", err)
	}
}

func runServiceTabFlow(t *testing.T) {
	t.Helper()
	requireChromeV1Integration(t)

	repoRoot, err := locateRepoRoot()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	pidPath := chromeServicePIDPath(repoRoot)
	if running, pid := serviceRunning(pidPath); running {
		t.Logf("chrome v1 service already running (pid=%d), stopping before test", pid)
		if err := runStop(nil); err != nil {
			t.Fatalf("stop existing service: %v", err)
		}
	}

	host := "127.0.0.1"
	httpPort := mustFreePort(t)
	natsPort := mustFreePort(t)
	chromeDebugPort := mustFreePort(t)
	natsURL := fmt.Sprintf("nats://127.0.0.1:%d", natsPort)
	natsPrefix := "chrome.v1.test"

	startArgs := []string{
		"--host", host,
		"--port", strconv.Itoa(httpPort),
		"--nats-url", natsURL,
		"--nats-prefix", natsPrefix,
		"--embedded-nats",
		"--chrome-debug-port", strconv.Itoa(chromeDebugPort),
		"--headless",
		"--initial-url", "https://www.google.com",
	}
	if err := runStart(startArgs); err != nil {
		t.Fatalf("runStart: %v", err)
	}
	defer func() {
		if err := runStop(nil); err != nil {
			t.Logf("runStop cleanup: %v", err)
		}
	}()

	healthURL := fmt.Sprintf("http://%s:%d/healthz", host, httpPort)
	if err := waitForHealthz(healthURL, 10*time.Second); err != nil {
		t.Fatalf("service health check failed: %v", err)
	}

	baseOpts := natsControlOptions{
		natsURL:    natsURL,
		natsPrefix: natsPrefix,
		timeout:    20 * time.Second,
	}

	resp, err := requestNATSCommand(baseOpts, ".tab.list", controlRequest{})
	if err != nil {
		t.Fatalf("list initial tabs: %v", err)
	}
	if len(resp.Tabs) != 1 || resp.Tabs[0].Name != "main" {
		t.Fatalf("expected only main tab initially, got %+v", resp.Tabs)
	}
	initialTargetID := resp.Tabs[0].TargetID

	if _, err := requestNATSCommand(baseOpts, ".tab.open", controlRequest{
		Tab: "foobar",
		URL: "https://example.com",
	}); err != nil {
		t.Fatalf("open foobar tab: %v", err)
	}

	resp, err = requestNATSCommand(baseOpts, ".tab.list", controlRequest{})
	if err != nil {
		t.Fatalf("list tabs after open: %v", err)
	}
	if len(resp.Tabs) != 2 {
		t.Fatalf("expected 2 tabs after open, got %+v", resp.Tabs)
	}
	names := map[string]bool{}
	for _, item := range resp.Tabs {
		names[item.Name] = true
	}
	if !names["main"] || !names["foobar"] {
		t.Fatalf("expected main and foobar tabs, got %+v", resp.Tabs)
	}

	if _, err := requestNATSCommand(baseOpts, ".tab.goto", controlRequest{
		Tab: "main",
		URL: "https://dialtone.earth",
	}); err != nil {
		t.Fatalf("goto main tab: %v", err)
	}

	resp, err = requestNATSCommand(baseOpts, ".tab.list", controlRequest{})
	if err != nil {
		t.Fatalf("list tabs after goto: %v", err)
	}
	for _, item := range resp.Tabs {
		if item.Name == "main" {
			if item.TargetID != initialTargetID {
				t.Errorf("main tab target_id changed from %s to %s after goto (recovery triggered unnecessarily?)", initialTargetID, item.TargetID)
			}
		}
	}
}

func requireChromeV1Integration(t *testing.T) {
	t.Helper()
	if !*chromeV1Integration {
		t.Skip("pass -chrome-v1-integration to run chrome/v1 integration tests")
	}
}
