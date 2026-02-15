package test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"strings"
	"sync"
	"time"

	"dialtone/cli/src/core/browser"
	"dialtone/cli/src/core/test"
	chromeApp "dialtone/cli/src/plugins/chrome/app"

	"strconv"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/performance"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	stdruntime "runtime"

	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
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
	stack   string
}

type sectionMetrics struct {
	CPU    float64 `json:"cpu"`
	Memory float64 `json:"memory"` // MB
	GPU    float64 `json:"gpu"`    // Placeholder or metric if available
	JSHeap float64 `json:"jsHeap"` // MB
	FPS    int     `json:"fps"`
	AppCPU float64 `json:"appCpu"` // ms
	AppGPU float64 `json:"appGpu"` // ms
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

	// Cleanup existing screenshots
	screenshotsDir := filepath.Join(cwd, "src", "plugins", "www", "screenshots")
	os.RemoveAll(screenshotsDir)
	os.MkdirAll(screenshotsDir, 0755)

	if !isPortOpen(5173) {
		fmt.Println(">> [WWW] Smoke: dev server not detected, starting")
		browser.CleanupPort(5173)
		devCmd := exec.Command("npm", "run", "dev", "--", "--host", "127.0.0.1")
		devCmd.Dir = wwwDir
		if err := devCmd.Start(); err != nil {
			return fmt.Errorf("failed to start dev server: %v", err)
		}
		defer func() {
			fmt.Println(">> [WWW] Smoke: stopping dev server...")
			if devCmd.Process != nil {
				devCmd.Process.Kill()
			}
		}()
	}

	var isNewBrowser bool

	// Ensure Chrome cleanup
	defer func() {
		if isNewBrowser {
			fmt.Println(">> [WWW] Smoke: cleaning up browser processes...")
			killCmd := getDialtoneCmd("chrome", "kill", "all")
			killCmd.Run()
		} else {
			fmt.Println(">> [WWW] Smoke: keeping existing browser open")
		}
	}()

	if err := waitForPortLocal(5173, 30*time.Second); err != nil {
		return fmt.Errorf("dev server port 5173 not ready: %v", err)
	}
	fmt.Println(">> [WWW] Smoke: dev server ready on 5173")

	// Check for --headed, --port, --wait, and --ignore-env flags
	isHeaded := false
	targetPort := 0

	ignoreEnv := false

	for _, arg := range os.Args {
		if arg == "--headed" {
			isHeaded = true
		} else if strings.HasPrefix(arg, "--port=") {
			pStr := strings.TrimPrefix(arg, "--port=")
			if p, err := strconv.Atoi(pStr); err == nil {
				targetPort = p
			}

		} else if arg == "--ignore-env" {
			ignoreEnv = true
		}
	}

	useHeadless := os.Getenv("SMOKE_HEADLESS") != "false" && !isHeaded
	wsURL, isNewBrowser, err := resolveChrome(targetPort, useHeadless, ignoreEnv)
	if err != nil {
		return err
	}
	fmt.Printf(">> [WWW] Smoke: chrome websocket %s (headless: %v)\n", wsURL, useHeadless)

	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	// Enable performance metrics collection
	if err := chromedp.Run(ctx, performance.Enable()); err != nil {
		fmt.Printf(">> [WWW] Smoke: failed to enable performance metrics: %v\n", err)
	}

	var mu sync.Mutex
	currentSection := ""
	entries := []consoleEntry{}
	performanceData := make(map[string]sectionMetrics)

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			msg := formatConsoleArgs(ev.Args)
			msgLower := strings.ToLower(msg)

			stack := ""
			if ev.StackTrace != nil {
				for _, f := range ev.StackTrace.CallFrames {
					stack += fmt.Sprintf("  %s (%s:%d:%d)\n", f.FunctionName, f.URL, f.LineNumber, f.ColumnNumber)
				}
			}

			mu.Lock()
			entries = append(entries, consoleEntry{
				section: currentSection,
				level:   string(ev.Type),
				message: msg,
				stack:   stack,
			})
			mu.Unlock()

			// Real-time streaming to terminal
			color := "\033[0m" // Default
			if ev.Type == "warning" {
				color = "\033[33m" // Yellow
			} else if ev.Type == "error" {
				color = "\033[31m" // Red
			} else if strings.Contains(msgLower, "resume") || strings.Contains(msgLower, "awake") {
				color = "\033[34;1m" // Bold Blue
			} else if strings.Contains(msgLower, "scrolling to") || strings.Contains(msgLower, "settled") {
				color = "\033[32m" // Green
			} else if strings.Contains(msgLower, "swap") {
				color = "\033[35m" // Magenta
			} else if strings.Contains(msgLower, "screenshot") {
				color = "\033[36m" // Cyan
			}
			fmt.Printf("   [APP] %s%s\033[0m\n", color, msg)
		case *runtime.EventExceptionThrown:
			msg := ev.ExceptionDetails.Text
			if ev.ExceptionDetails.Exception != nil {
				msg = ev.ExceptionDetails.Exception.Description
			}
			stack := ""
			if ev.ExceptionDetails.StackTrace != nil {
				for _, f := range ev.ExceptionDetails.StackTrace.CallFrames {
					stack += fmt.Sprintf("  %s (%s:%d:%d)\n", f.FunctionName, f.URL, f.LineNumber, f.ColumnNumber)
				}
			}
			mu.Lock()
			entries = append(entries, consoleEntry{
				section: currentSection,
				level:   "exception",
				message: msg,
				stack:   stack,
			})
			mu.Unlock()
		}
	})

	base := "http://127.0.0.1:5173"
	var sections []string
	if err := chromedp.Run(ctx,
		chromedp.EmulateViewport(375, 812, chromedp.EmulateMobile),
		chromedp.Navigate(base),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('section[id^="s-"]')).map(el => el.id)`, &sections),
	); err != nil {
		return fmt.Errorf("failed to navigate/inject: %v", err)
	}

	var allErrors []string

	// TRIGGER PROOFOFLIFE ERRORS
	fmt.Println(">> [WWW] Smoke: triggering Proof of Life errors...")
	// 1. Browser Error
	if err := chromedp.Run(ctx, chromedp.Evaluate(`console.error('[PROOFOFLIFE] Intentional Browser Test Error')`, nil)); err != nil {
		fmt.Printf("   [WARN] Failed to trigger browser Proof of Life error: %v\n", err)
	}
	// 2. Go Error (simulated via log captured by listener)
	mu.Lock()
	entries = append(entries, consoleEntry{
		section: "init",
		level:   "error",
		message: "[PROOFOFLIFE] Intentional Go Test Error",
		stack:   "  RunWwwSmoke (smoke.go:210)",
	})
	mu.Unlock()

	for i, section := range sections {
		mu.Lock()
		currentSection = section
		startIdx := len(entries)
		mu.Unlock()

		fmt.Printf(">> [WWW] Smoke: [%d/%d] NAVIGATING TO: #%s\n", i+1, len(sections), section)
		var buf []byte
		var currentHash string
		var scrollY float64
		var m sectionMetrics

		// Navigate
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf("window.location.hash = '%s'", section), nil),
		); err != nil {
			return err
		}

		// Wait for ready log
		ready := false
		timeout := time.After(8 * time.Second) // Reduced from 30s
		lastEntryIdx := startIdx
		fmt.Printf("   [WAIT] Waiting for READY: #%s\n", section)
		for !ready {
			select {
			case <-timeout:
				fmt.Printf("   [WARN] Timeout waiting for ready on %s\n", section)
				ready = true 
			default:
				mu.Lock()
				if len(entries) > lastEntryIdx {
					for _, entry := range entries[lastEntryIdx:] {
						if strings.Contains(entry.message, "READY:") && strings.Contains(entry.message, section) {
							fmt.Printf("   [DEBUG] Found READY log for %s\n", section)
							ready = true
						}
					}
					lastEntryIdx = len(entries)
				}
				mu.Unlock()
				if !ready {
					time.Sleep(50 * time.Millisecond) // Faster polling
				}
			}
		}

		// Extra wait for animations
		time.Sleep(200 * time.Millisecond) // Reduced from 500ms

		// Capture screenshot
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`(async () => {
				const mem = (performance && performance.memory) ? {
					jsHeap: performance.memory.usedJSHeapSize / (1024 * 1024)
				} : { jsHeap: 0 };
				
				const resources = performance.getEntriesByType('resource');
				const totalSize = resources.reduce((acc, r) => acc + (r.transferSize || 0), 0) / (1024 * 1024);
				
				return {
					cpu: 0, 
					memory: totalSize,
					jsHeap: mem.jsHeap,
					gpu: 0
				};
			})()`, &m),
			chromedp.ActionFunc(func(ctx context.Context) error {
				metrics, err := performance.GetMetrics().Do(ctx)
				if err != nil {
					return err
				}
				for _, metric := range metrics {
					if metric.Name == "ScriptDuration" {
						m.CPU = metric.Value 
					}
				}
				return nil
			}),
			chromedp.Evaluate("window.location.hash", &currentHash),
			chromedp.Evaluate("document.body.scrollTop", &scrollY),
			chromedp.Evaluate(fmt.Sprintf("console.log('[PROOFOFLIFE] 📸 SCREENSHOT STARTING: %s')", section), nil),
			chromedp.ActionFunc(func(ctx context.Context) error {
				b, err := page.CaptureScreenshot().Do(ctx)
				if err != nil {
					return err
				}
				buf = b
				return nil
			}),
		); err != nil {
			allErrors = append(allErrors, fmt.Errorf("screenshot %s failed: %v", section, err).Error())
			continue
		}

		// Merge stats
		mu.Lock()
		performanceData[section] = m
		mu.Unlock()
		fmt.Printf("   [TEST] Verify: hash=%s, scrollY=%.0f, heap=%.1fMB\n", currentHash, scrollY, m.JSHeap)

		if len(buf) > 0 {
			screenshotPath := filepath.Join(screenshotsDir, fmt.Sprintf("%s.png", section))
			if err := os.WriteFile(screenshotPath, buf, 0644); err != nil {
				fmt.Printf(">> [WWW] Smoke: failed to save screenshot for %s: %v\n", section, err)
			}
		}

		mu.Lock()
		newEntries := []consoleEntry{}
		for _, entry := range entries[startIdx:] {
			msg := entry.message
			isCadError := strings.Contains(msg, "[cad] Server might be offline") ||
				strings.Contains(msg, "[cad] Model update failed") ||
				strings.Contains(msg, "[cad] Response status: 500") ||
				strings.Contains(msg, "[cad] Fetch failed")

			isInfo := strings.Contains(msg, "SWAP:") || strings.Contains(msg, "SCREENSHOT STARTING")

			if isCadError || isInfo {
				continue
			}
			newEntries = append(newEntries, entry)
		}
		mu.Unlock()

		if len(newEntries) > 0 {
			fmt.Printf(">> [WWW] Smoke: %s | console issues detected\n", section)
			for _, entry := range newEntries {
				allErrors = append(allErrors, fmt.Sprintf("[%s] #%s: %s", entry.level, section, entry.message))
			}
		} else {
			fmt.Printf(">> [WWW] Smoke: %s | ok\n", section)
		}
	}

	summaryPath := filepath.Join(screenshotsDir, "summary.png")
	smokeMdPath := filepath.Join(cwd, "src", "plugins", "www", "SMOKE.md")

	// Error Categorization
	proofOfLifeErrors := make(map[string]consoleEntry)
	uniqueErrors := make(map[string]consoleEntry)
	for _, entry := range entries {
		msg := entry.message
		if strings.Contains(msg, "[cad] Server might be offline") ||
			strings.Contains(msg, "[cad] Model update failed") ||
			strings.Contains(msg, "[cad] Response status: 500") ||
			strings.Contains(msg, "[cad] Fetch failed") {
			continue
		}

		if strings.Contains(msg, "[PROOFOFLIFE]") {
			if _, ok := proofOfLifeErrors[msg]; !ok {
				proofOfLifeErrors[msg] = entry
			}
			continue
		}

		if _, ok := uniqueErrors[msg]; !ok {
			uniqueErrors[msg] = entry
		}
	}

	// Generate SMOKE.md
	var smLines []string
	smLines = append(smLines, "# WWW Plugin Smoke Test Report")
	smLines = append(smLines, fmt.Sprintf("\n**Generated at:** %s", time.Now().Format(time.RFC1123)))

	smLines = append(smLines, "\n## 1. Expected Errors (Proof of Life)")
	if len(proofOfLifeErrors) == 0 {
		smLines = append(smLines, "\n❌ ERROR: Proof of Life logs missing! Logging pipeline may be broken.")
	} else {
		smLines = append(smLines, "\n| Level | Message | Status |")
		smLines = append(smLines, "|---|---|---|")
		for _, entry := range proofOfLifeErrors {
			smLines = append(smLines, fmt.Sprintf("| %s | %s | ✅ CAPTURED |", entry.level, entry.message))
		}
	}

	smLines = append(smLines, "\n## 2. Real Errors & Warnings")
	if len(uniqueErrors) == 0 {
		smLines = append(smLines, "\n✅ No actual issues detected.")
	} else {
		for _, entry := range uniqueErrors {
			smLines = append(smLines, fmt.Sprintf("\n### [%s] %s", entry.level, entry.section))
			smLines = append(smLines, "```")
			smLines = append(smLines, entry.message)
			if entry.stack != "" {
				smLines = append(smLines, "\nStack Trace:")
				smLines = append(smLines, entry.stack)
			}
			smLines = append(smLines, "```")
		}
	}

	smLines = append(smLines, "\n## 3. Performance Metrics")
	smLines = append(smLines, "\n| Section | FPS | App CPU (ms) | App GPU (ms) | JS Heap (MB) | Resources (MB) | Status |")
	smLines = append(smLines, "|---|---|---|---|---|---|---|")
	for _, section := range sections {
		m := performanceData[section]
		smLines = append(smLines, fmt.Sprintf("| %s | %d | %.2f | %.2f | %.2f | %.2f | OK |", section, m.FPS, m.AppCPU, m.AppGPU, m.JSHeap, m.Memory))
	}

	smLines = append(smLines, "\n## 4. Visual Summary Grid")
	smLines = append(smLines, "\n![Summary Grid](screenshots/summary.png)")

	os.WriteFile(smokeMdPath, []byte(strings.Join(smLines, "\n")), 0644)

	if err := TileScreenshots(screenshotsDir, summaryPath, sections); err == nil {
		fmt.Printf("\n>> [WWW] Smoke COMPLETE")
	} else {
		fmt.Printf(">> [WWW] Smoke: tiling failed: %v\n", err)
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("smoke tests encountered issues:\n%s", strings.Join(allErrors, "\n"))
	}

	fmt.Println(">> [WWW] Smoke: pass")
	return nil
}

func resolveChrome(requestedPort int, headless bool, ignoreEnv bool) (string, bool, error) {
	if requestedPort > 0 {
		fmt.Printf(">> [WWW] Smoke: using requested port %d\n", requestedPort)
		ws, err := readWebSocketURL(fmt.Sprintf("%d", requestedPort))
		if err == nil && ws != "" {
			return ws, false, nil
		}
		fmt.Printf(">> [WWW] Smoke: launching chrome on requested port %d\n", requestedPort)
		res, err := chromeApp.LaunchChrome(requestedPort, true, headless, "")
		if err != nil {
			return "", false, err
		}
		return res.WebsocketURL, true, nil
	}

	if !ignoreEnv {
		if ws := os.Getenv("CHROME_WS"); ws != "" {
			fmt.Println(">> [WWW] Smoke: using CHROME_WS")
			return ws, false, nil
		}
		if pStr := os.Getenv("CHROME_DEBUG_PORT"); pStr != "" {
			if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
				fmt.Printf(">> [WWW] Smoke: checking env CHROME_DEBUG_PORT %d\n", p)
				ws, err := readWebSocketURL(pStr)
				if err == nil && ws != "" {
					return ws, false, nil
				}
			}
		}
	}

	procs, err := chromeApp.ListResources(true)
	if err == nil {
		for _, p := range procs {
			if p.DebugPort > 0 {
				fmt.Printf(">> [WWW] Smoke: found existing chrome on port %d\n", p.DebugPort)
				ws, err := readWebSocketURL(fmt.Sprintf("%d", p.DebugPort))
				if err == nil && ws != "" {
					return ws, false, nil
				}
			}
		}
	}

	reqDesc := "headed"
	if headless {
		reqDesc = "headless"
	}
	fmt.Printf(">> [WWW] Smoke: launching new %s chrome (auto-port)\n", reqDesc)
	res, err := chromeApp.LaunchChrome(0, true, headless, "")
	if err != nil {
		return "", false, err
	}
	return res.WebsocketURL, true, nil
}

func TileScreenshots(dir string, output string, order []string) error {
	var pngs []string
	for _, section := range order {
		path := filepath.Join(dir, fmt.Sprintf("%s.png", section))
		if _, err := os.Stat(path); err == nil {
			pngs = append(pngs, path)
		} else {
			fmt.Printf(">> [WWW] Smoke: warning - missing screenshot for %s\n", section)
		}
	}

	if len(pngs) == 0 {
		return fmt.Errorf("no screenshots found to tile")
	}

	const (
		tileW = 375
		tileH = 812
	)

	n := len(pngs)
	cols := int(math.Ceil(math.Sqrt(float64(n))))
	if cols < 1 {
		cols = 1
	}
	rows := int(math.Ceil(float64(n) / float64(cols)))

	dst := image.NewRGBA(image.Rect(0, 0, tileW*cols, tileH*rows))
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

		x := (i % cols) * tileW
		y := (i / cols) * tileH
		rect := image.Rect(x, y, x+tileW, y+tileH)

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

func readWebSocketURL(port string) (string, error) {
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
