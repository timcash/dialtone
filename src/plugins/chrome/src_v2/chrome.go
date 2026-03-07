package src_v2

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

var (
	mu             sync.Mutex
	globalAllocCtx context.Context
	globalTabCtx   context.Context
)

type OpenRequest struct {
	URL    string `json:"url"`
	NewTab bool   `json:"new_tab"`
}

type ChromeStats struct {
	ChromePID   int      `json:"chrome_pid"`
	ServicePID  int      `json:"service_pid"`
	ChromePort  int      `json:"chrome_port"`
	NATSPort    int      `json:"nats_port"`
	TabCount    int      `json:"tab_count"`
	ProfilePath string   `json:"profile_path"`
	CacheSizeKB int64    `json:"cache_size_kb"`
	Tabs        []string `json:"tabs"`
}

func Start(port int, req OpenRequest) error {
	mu.Lock()
	defer mu.Unlock()

	// 1. Ensure Process
	if !isPortOpen(port) {
		fmt.Printf("[DEBUG] Port %d closed. Launching...\n", port)
		if err := launchProcess(port); err != nil { return err }
		globalAllocCtx = nil
		globalTabCtx = nil
		for i := 0; i < 20; i++ {
			if isPortOpen(port) { break }
			time.Sleep(500 * time.Millisecond)
		}
	}

	// 2. Ensure Allocator
	if globalAllocCtx == nil {
		wsURL := fmt.Sprintf("ws://127.0.0.1:%d", port)
		fmt.Printf("[DEBUG] Allocator -> %s\n", wsURL)
		globalAllocCtx, _ = chromedp.NewRemoteAllocator(context.Background(), wsURL)
	}

	// 3. Ensure Tab Context (Don't hang on Run)
	if globalTabCtx == nil {
		fmt.Println("[DEBUG] Locating primary tab...")
		var firstID string
		for i := 0; i < 10; i++ {
			firstID = findFirstTab(port)
			if firstID != "" { break }
			time.Sleep(500 * time.Millisecond)
		}

		if firstID != "" {
			fmt.Printf("[DEBUG] Attached -> %s\n", firstID)
			globalTabCtx, _ = chromedp.NewContext(globalAllocCtx, chromedp.WithTargetID(target.ID(firstID)))
		} else {
			fmt.Println("[DEBUG] No existing tab, new context")
			globalTabCtx, _ = chromedp.NewContext(globalAllocCtx)
		}
		// ESSENTIAL: Initialize the context once
		_ = chromedp.Run(globalTabCtx, chromedp.ActionFunc(func(ctx context.Context) error { return nil }))
	}

	// 4. Navigate
	if req.URL == "" { return nil }
	
	ctx := globalTabCtx
	if req.NewTab {
		fmt.Printf("[DEBUG] New Tab -> %s\n", req.URL)
		ctx, _ = chromedp.NewContext(globalAllocCtx)
	} else {
		fmt.Printf("[DEBUG] Navigate -> %s\n", req.URL)
	}

	// Use a timeout for the actual navigation to avoid hanging the service
	runCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	
	return chromedp.Run(runCtx, chromedp.Navigate(req.URL))
}

func isPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	if err != nil { return false }
	conn.Close()
	return true
}

func launchProcess(port int) error {
	path := FindChromePath()
	userDataDir, _ := filepath.Abs(".chrome_data_v2")
	_ = os.MkdirAll(userDataDir, 0755)
	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--user-data-dir=" + userDataDir,
		"--no-first-run",
		"--no-default-browser-check",
		"about:blank",
	}
	return exec.Command(path, args...).Start()
}

func findFirstTab(port int) string {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err != nil { return "" }
	defer resp.Body.Close()
	var targets []map[string]any
	json.NewDecoder(resp.Body).Decode(&targets)
	for _, t := range targets {
		if t["type"] == "page" { return t["id"].(string) }
	}
	return ""
}

func GetStats(chromePort, natsPort int) ChromeStats {
	stats := ChromeStats{ServicePID: os.Getpid(), ChromePort: chromePort, NATSPort: natsPort}
	profile, _ := filepath.Abs(".chrome_data_v2")
	stats.ProfilePath = profile
	filepath.Walk(profile, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() { stats.CacheSizeKB += info.Size() }
		return nil
	})
	stats.CacheSizeKB /= 1024
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", chromePort))
	if err == nil {
		defer resp.Body.Close()
		var targets []map[string]any
		json.NewDecoder(resp.Body).Decode(&targets)
		for _, t := range targets {
			if t["type"] == "page" {
				stats.TabCount++
				stats.Tabs = append(stats.Tabs, fmt.Sprintf("%v", t["url"]))
			}
		}
	}
	return stats
}

func StartService(natsPort, chromePort, wsPort int) error {
	ns, _ := server.NewServer(&server.Options{Port: natsPort})
	go ns.Start()
	
	// Initial warmup (async)
	go func() {
		time.Sleep(1 * time.Second)
		_ = Start(chromePort, OpenRequest{URL: ""})
	}()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil { return }
		defer c.Close(websocket.StatusInternalError, "closing")
		for {
			stats := GetStats(chromePort, natsPort)
			if err := wsjson.Write(r.Context(), c, stats); err != nil { break }
			time.Sleep(2 * time.Second)
		}
	})
	go http.ListenAndServe(fmt.Sprintf(":%d", wsPort), nil)

	nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", natsPort))
	if err != nil { return err }
	nc.Subscribe("chrome.open", func(m *nats.Msg) {
		var req OpenRequest
		json.Unmarshal(m.Data, &req)
		if req.URL == "" { req.URL = "about:blank" }
		fmt.Printf("NATS: Open %s\n", req.URL)
		_ = Start(chromePort, req)
	})

	fmt.Printf("Service listening. Subjects: chrome.open\n")
	select {}
}

func FindChromePath() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("ProgramFiles") + "\\Google\\Chrome\\Application\\chrome.exe"
	}
	if runtime.GOOS == "darwin" {
		return "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	}
	return "google-chrome"
}
