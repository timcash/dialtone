package src_v2

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestChromeTabManagementNATS(t *testing.T) {
	natsPort := 4560
	chromePort := 9671

	// Start Service
	go func() {
		_ = StartService(natsPort, chromePort)
	}()
	time.Sleep(2 * time.Second)

	nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", natsPort))
	if err != nil {
		t.Fatalf("failed to connect to nats: %v", err)
	}
	defer nc.Close()

	open := func(url string, newTab bool) {
		req := OpenRequest{URL: url, NewTab: newTab}
		data, _ := json.Marshal(req)
		_ = nc.Publish("chrome.open", data)
		time.Sleep(10 * time.Second)
	}

	getTabCount := func() int {
		msg, err := nc.Request("chrome.status", nil, 2*time.Second)
		if err != nil {
			t.Fatalf("NATS request failed: %v", err)
		}
		var res StatusResponse
		json.Unmarshal(msg.Data, &res)
		return res.TabCount
	}

	// 1. Cold start
	t.Log("Step 1: Cold start")
	open("https://www.google.com", false)
	if count := getTabCount(); count != 1 {
		t.Errorf("Expected 1 tab, got %d", count)
	}

	// 2. New Tab
	t.Log("Step 2: Open second tab")
	open("https://dialtone.earth", true)
	if count := getTabCount(); count != 2 {
		t.Errorf("Expected 2 tabs, got %d", count)
	}

	// 3. Third Tab
	t.Log("Step 3: Open third tab")
	open("https://ipchicken.com", true)
	if count := getTabCount(); count != 3 {
		t.Errorf("Expected 3 tabs, got %d", count)
	}
}
