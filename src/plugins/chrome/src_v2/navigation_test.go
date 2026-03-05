package src_v2

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestNavigationVerification(t *testing.T) {
	natsPort := 4562
	chromePort := 9673
	wsPort := 19447

	// 0. Cleanup any process on these ports
	exec.Command("powershell.exe", "-Command", fmt.Sprintf("Get-NetTCPConnection -LocalPort %d -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }", chromePort)).Run()

	// 1. Start Service
	go func() {
		_ = StartService(natsPort, chromePort, wsPort)
	}()
	time.Sleep(10 * time.Second) // Wait for cold start (5s in Start + safety)

	nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", natsPort))
	if err != nil {
		t.Fatalf("failed to connect to nats: %v", err)
	}
	defer nc.Close()

	// 2. Verify we start on about:blank (warmup behavior)
	currentURL := getFirstTabURL(t, chromePort)
	if currentURL != "about:blank" {
		t.Logf("Note: Started on %s instead of about:blank", currentURL)
	}

	// 3. Send Navigation Request
	targetURL := "https://example.com/"
	req := OpenRequest{URL: targetURL}
	data, _ := json.Marshal(req)
	_ = nc.Publish("chrome.open", data)

	// 4. Poll for Navigation Success
	success := false
	for i := 0; i < 10; i++ {
		time.Sleep(2 * time.Second)
		currentURL = getFirstTabURL(t, chromePort)
		t.Logf("Attempt %d: Current URL is %s", i+1, currentURL)
		if currentURL == targetURL || currentURL == "https://www.example.com/" {
			success = true
			break
		}
	}

	if !success {
		t.Errorf("Failed to navigate. Final URL: %s", currentURL)
	}
}

func getFirstTabURL(t *testing.T, port int) string {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err != nil {
		t.Fatalf("failed to get targets: %v", err)
	}
	defer resp.Body.Close()
	var targets []map[string]any
	json.NewDecoder(resp.Body).Decode(&targets)
	for _, t := range targets {
		if t["type"] == "page" {
			return t["url"].(string)
		}
	}
	return ""
}
