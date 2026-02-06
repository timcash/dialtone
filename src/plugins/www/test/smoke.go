package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"dialtone/cli/src/core/browser"
	"dialtone/cli/src/core/test"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func init() {
	test.Register("www-smoke", "www", []string{"www", "smoke", "browser"}, RunWwwSmoke)
}

type consoleEntry struct {
	section string
	level   string
	message string
}

// RunWwwSmoke starts the dev server and quickly checks each section for warnings/errors.
func RunWwwSmoke() error {
	fmt.Println(">> [WWW] Smoke: start")
	cwd, _ := os.Getwd()
	dialtoneSh := filepath.Join(cwd, "dialtone.sh")
	if _, err := os.Stat(dialtoneSh); os.IsNotExist(err) {
		return fmt.Errorf("could not find dialtone.sh in %s", cwd)
	}

	wwwDir := filepath.Join(cwd, "src", "plugins", "www", "app")
	if _, err := os.Stat(wwwDir); os.IsNotExist(err) {
		return fmt.Errorf("www app directory not found: %s", wwwDir)
	}

	startedDev := false
	if !isPortOpen(5173) {
		fmt.Println(">> [WWW] Smoke: dev server not detected, starting")
		browser.CleanupPort(5173)
		devCmd := exec.Command("npm", "run", "dev", "--", "--host", "127.0.0.1")
		devCmd.Dir = wwwDir
		if err := devCmd.Start(); err != nil {
			return fmt.Errorf("failed to start dev server: %v", err)
		}
		startedDev = true
		defer func() {
			if devCmd.Process != nil {
				devCmd.Process.Kill()
			}
		}()
	}

	if err := waitForPortLocal(5173, 30*time.Second); err != nil {
		return fmt.Errorf("dev server port 5173 not ready: %v", err)
	}
	fmt.Println(">> [WWW] Smoke: dev server ready on 5173")

	wsURL, err := getChromeWebSocketURL()
	if err != nil {
		return err
	}
	fmt.Printf(">> [WWW] Smoke: chrome websocket %s\n", wsURL)

	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	var mu sync.Mutex
	currentSection := ""
	entries := []consoleEntry{}

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			if ev.Type != "warning" && ev.Type != "error" {
				return
			}
			msg := formatConsoleArgs(ev.Args)
			mu.Lock()
			entries = append(entries, consoleEntry{
				section: currentSection,
				level:   string(ev.Type),
				message: msg,
			})
			mu.Unlock()
		case *runtime.EventExceptionThrown:
			msg := ev.ExceptionDetails.Text
			mu.Lock()
			entries = append(entries, consoleEntry{
				section: currentSection,
				level:   "exception",
				message: msg,
			})
			mu.Unlock()
		}
	})

	sections := []string{
		"s-home",
		"s-robot",
		"s-neural",
		"s-math",
		"s-cad",
		"s-about",
		"s-radio",
		"s-geotools",
		"s-docs",
		"s-webgpu-template",
		"s-threejs-template",
	}

	base := "http://127.0.0.1:5173"
	for _, section := range sections {
		mu.Lock()
		currentSection = section
		startIdx := len(entries)
		mu.Unlock()

		fmt.Printf(">> [WWW] Smoke: navigate #%s\n", section)
		if err := chromedp.Run(ctx,
			chromedp.Navigate(fmt.Sprintf("%s/#%s", base, section)),
			chromedp.Sleep(500*time.Millisecond),
		); err != nil {
			return fmt.Errorf("navigate %s failed: %v", section, err)
		}

		mu.Lock()
		newEntries := append([]consoleEntry{}, entries[startIdx:]...)
		mu.Unlock()
		if len(newEntries) > 0 {
			fmt.Printf(">> [WWW] Smoke: console issues in #%s\n", section)
			var lines []string
			for _, entry := range newEntries {
				lines = append(lines, fmt.Sprintf("[%s] %s", entry.level, entry.message))
			}
			return fmt.Errorf("console warnings/errors in %s:\n%s", section, strings.Join(lines, "\n"))
		}
		fmt.Printf(">> [WWW] Smoke: ok #%s\n", section)
	}

	if startedDev {
		fmt.Println(">> [WWW] Smoke complete, stopping dev server.")
	}
	fmt.Println(">> [WWW] Smoke: pass")
	return nil
}

func formatConsoleArgs(args []*runtime.RemoteObject) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == nil {
			continue
		}
		if len(arg.Value) > 0 {
			var v interface{}
			if err := json.Unmarshal(arg.Value, &v); err == nil {
				b, err := json.Marshal(v)
				if err == nil {
					parts = append(parts, string(b))
					continue
				}
			}
			parts = append(parts, string(arg.Value))
			continue
		}
		if arg.Description != "" {
			parts = append(parts, arg.Description)
			continue
		}
		parts = append(parts, string(arg.Type))
	}
	return strings.Join(parts, " ")
}

func getChromeWebSocketURL() (string, error) {
	if ws := os.Getenv("CHROME_WS"); ws != "" {
		fmt.Println(">> [WWW] Smoke: using CHROME_WS")
		return ws, nil
	}

	port := os.Getenv("CHROME_DEBUG_PORT")
	if port == "" {
		port = "9222"
	}
	fmt.Printf(">> [WWW] Smoke: checking chrome debug port %s\n", port)
	if ws, err := readWebSocketURL(port); err == nil && ws != "" {
		fmt.Println(">> [WWW] Smoke: attached to existing chrome")
		return ws, nil
	}

	fmt.Println(">> [WWW] Smoke: launching chrome")
	launchCmd := exec.Command("./dialtone.sh", "chrome", "new", "--gpu")
	output, err := launchCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to launch chrome: %v\nOutput: %s", err, string(output))
	}

	re := regexp.MustCompile(`ws://127\.0\.0\.1:\d+/devtools/browser/[a-z0-9-]+`)
	wsURL := re.FindString(string(output))
	if wsURL == "" {
		return "", fmt.Errorf("failed to parse chrome WebSocket URL: %s", string(output))
	}
	return wsURL, nil
}

func readWebSocketURL(port string) (string, error) {
	fmt.Println(">> [WWW] Smoke: fetching /json/version")
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s/json/version", port))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var payload struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return payload.WebSocketDebuggerURL, nil
}

func waitForPortLocal(port int, timeout time.Duration) error {
	start := time.Now()
	for time.Since(start) < timeout {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for port %d", port)
}

func isPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
