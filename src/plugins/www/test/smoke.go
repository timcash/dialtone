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
	stdruntime "runtime"

	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"sort"
)

func getDialtoneCmd(args ...string) *exec.Cmd {
	if stdruntime.GOOS == "windows" {
		return exec.Command("powershell", append([]string{"-ExecutionPolicy", "Bypass", "-File", ".\\dialtone.ps1"}, args...)...)
	}
	return exec.Command("./dialtone.sh", args...)
}

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
	dialtoneScript := filepath.Join(cwd, "dialtone.sh")
	if stdruntime.GOOS == "windows" {
		dialtoneScript = filepath.Join(cwd, "dialtone.ps1")
	}
	if _, err := os.Stat(dialtoneScript); os.IsNotExist(err) {
		return fmt.Errorf("could not find dialtone script in %s", cwd)
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

	base := "http://127.0.0.1:5173"
	var sections []string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(base),
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('section[id^="s-"]')).map(el => el.id)`, &sections),
	); err != nil {
		return fmt.Errorf("failed to fetch section IDs: %v", err)
	}

	for _, section := range sections {
		mu.Lock()
		currentSection = section
		startIdx := len(entries)
		mu.Unlock()

		fmt.Printf(">> [WWW] Smoke: navigate #%s\n", section)
		var buf []byte
		if err := chromedp.Run(ctx,
			chromedp.Navigate(fmt.Sprintf("%s/#%s", base, section)),
			chromedp.Sleep(800*time.Millisecond), // Slightly longer wait for rendering
			chromedp.FullScreenshot(&buf, 90),
		); err != nil {
			return fmt.Errorf("navigate %s failed: %v", section, err)
		}

		if len(buf) > 0 {
			screenshotsDir := filepath.Join(cwd, "src", "plugins", "www", "screenshots")
			os.MkdirAll(screenshotsDir, 0755)
			screenshotPath := filepath.Join(screenshotsDir, fmt.Sprintf("%s.png", section))
			if err := os.WriteFile(screenshotPath, buf, 0644); err != nil {
				fmt.Printf(">> [WWW] Smoke: failed to save screenshot for %s: %v\n", section, err)
			} else {
				fmt.Printf(">> [WWW] Smoke: saved screenshot to %s\n", screenshotPath)
			}
		}

		mu.Lock()
		newEntries := []consoleEntry{}
		for _, entry := range entries[startIdx:] {
			// Skip expected CAD backend errors if server is offline.
			// These can arrive late (e.g. during a subsequent section) so we filter them globally.
			isCadError := strings.Contains(entry.message, "[cad] Server might be offline") ||
				strings.Contains(entry.message, "[cad] Model update failed")
			if isCadError {
				continue
			}
			newEntries = append(newEntries, entry)
		}
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

	screenshotsDir := filepath.Join(cwd, "src", "plugins", "www", "screenshots")
	summaryPath := filepath.Join(screenshotsDir, "summary.png")
	if err := TileScreenshots(screenshotsDir, summaryPath); err != nil {
		fmt.Printf(">> [WWW] Smoke: tiling failed: %v\n", err)
	}

	fmt.Println(">> [WWW] Smoke: pass")
	return nil
}

func TileScreenshots(dir string, output string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var pngs []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".png") && f.Name() != "summary.png" {
			pngs = append(pngs, filepath.Join(dir, f.Name()))
		}
	}
	sort.Strings(pngs)

	if len(pngs) == 0 {
		return fmt.Errorf("no screenshots found to tile")
	}

	// Limit to 9 for 3x3
	if len(pngs) > 9 {
		pngs = pngs[:9]
	}

	const (
		tileW = 400
		tileH = 300 // typical aspect ratio
		grid  = 3
	)

	dst := image.NewRGBA(image.Rect(0, 0, tileW*grid, tileH*grid))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{image.Black}, image.Point{}, draw.Src)

	for i, path := range pngs {
		f, err := os.Open(path)
		if err != nil {
			fmt.Printf(">> [WWW] Smoke: failed to open %s: %v\n", path, err)
			continue
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			fmt.Printf(">> [WWW] Smoke: failed to decode %s: %v\n", path, err)
			continue
		}

		x := (i % grid) * tileW
		y := (i / grid) * tileH
		rect := image.Rect(x, y, x+tileW, y+tileH)

		// Simple nearest neighbor "resize" by drawing into the grid slot
		// Note: draw.Draw doesn't scale. We need a scaling draw call.
		// Since we don't have x/image/draw, we'll implement a tiny scaler.
		drawTile(dst, rect, img)
	}

	out, err := os.Create(output)
	if err != nil {
		return err
	}
	defer out.Close()

	return png.Encode(out, dst)
}

func drawTile(dst *image.RGBA, rect image.Rectangle, src image.Image) {
	sw := src.Bounds().Dx()
	sh := src.Bounds().Dy()
	dw := rect.Dx()
	dh := rect.Dy()

	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			sx := (x * sw) / dw
			sy := (y * sh) / dh
			dst.Set(rect.Min.X+x, rect.Min.Y+y, src.At(sx, sy))
		}
	}
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
	launchCmd := getDialtoneCmd("chrome", "new", "--gpu")
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
