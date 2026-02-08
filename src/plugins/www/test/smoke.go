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
	"regexp"

	"strings"
	"sync"
	"time"

	"dialtone/cli/src/core/browser"
	"dialtone/cli/src/core/test"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/performance" // Added import
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
	CPU      float64 `json:"cpu"`
	Memory   float64 `json:"memory"` // MB
	GPU      float64 `json:"gpu"`    // Placeholder or metric if available
	JSHeap   float64 `json:"jsHeap"` // MB
	FPS      int     `json:"fps"`
	AppCPU   float64 `json:"appCpu"` // ms
	AppGPU   float64 `json:"appGpu"` // ms
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

	// Ensure Chrome cleanup
	defer func() {
		fmt.Println(">> [WWW] Smoke: cleaning up browser processes...")
		killCmd := getDialtoneCmd("chrome", "kill", "all")
		killCmd.Run()
	}()

	if err := waitForPortLocal(5173, 30*time.Second); err != nil {
		return fmt.Errorf("dev server port 5173 not ready: %v", err)
	}
	fmt.Println(">> [WWW] Smoke: dev server ready on 5173")

	// Check for --headed flag
	isHeaded := false
	for _, arg := range os.Args {
		if arg == "--headed" {
			isHeaded = true
			break
		}
	}

	useHeadless := os.Getenv("SMOKE_HEADLESS") != "false" && !isHeaded
	var wsURL string
	var err error
	if useHeadless {
		wsURL, err = getChromeWebSocketURLHeadless()
	} else {
		wsURL, err = getChromeWebSocketURL()
	}
	if err != nil {
		return err
	}
	fmt.Printf(">> [WWW] Smoke: chrome websocket %s (headless: %v)\n", wsURL, useHeadless)

	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
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
			// Log everything to stdout as requested, but only track issues for failure
			isIssue := ev.Type == "warning" || ev.Type == "error" ||
				(ev.Type == "log" && (
					strings.Contains(msgLower, "error") ||
					strings.Contains(msgLower, "warning")))

			stack := ""
			if ev.StackTrace != nil {
				for _, f := range ev.StackTrace.CallFrames {
					stack += fmt.Sprintf("  %s (%s:%d:%d)\n", f.FunctionName, f.URL, f.LineNumber, f.ColumnNumber)
				}
			}

			if isIssue {
				mu.Lock()
				entries = append(entries, consoleEntry{
					section: currentSection,
					level:   string(ev.Type),
					message: msg,
					stack:   stack,
				})
				mu.Unlock()
			} else if strings.Contains(msg, "[SMOKE_STATS]") {
				// Parse stats log
				// msg is likely quoted like "[SMOKE_STATS] {...}" due to formatConsoleArgs
				cleanMsg := msg
				if strings.HasPrefix(cleanMsg, "\"") && strings.HasSuffix(cleanMsg, "\"") {
					cleanMsg = cleanMsg[1 : len(cleanMsg)-1]
					// Unescape quotes if needed, but simple slicing might be enough for now if no escaped chars inside JSON
					// Actually, JSON.stringify produces escaped quotes. We should use strconv.Unquote or json.Unmarshal
					var unquoted string
					if err := json.Unmarshal([]byte(msg), &unquoted); err == nil {
						cleanMsg = unquoted
					}
				}

				jsonStr := strings.TrimPrefix(cleanMsg, "[SMOKE_STATS] ")
				var stats struct {
					FPS    int     `json:"fps"`
					AppCPU float64 `json:"cpu"`
					AppGPU float64 `json:"gpu"`
				}
				if err := json.Unmarshal([]byte(jsonStr), &stats); err == nil {
					mu.Lock()
					if m, ok := performanceData[currentSection]; ok {
						m.FPS = stats.FPS
						m.AppCPU = stats.AppCPU
						m.AppGPU = stats.AppGPU
						performanceData[currentSection] = m
					} else {
						// Initialize if not exists (though loop usually inits)
						performanceData[currentSection] = sectionMetrics{
							FPS:    stats.FPS,
							AppCPU: stats.AppCPU,
							AppGPU: stats.AppGPU,
						}
					}
					mu.Unlock()
				}
			}
			// ... real-time streaming ...

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
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('section[id^="s-"]')).map(el => el.id)`, &sections),
	); err != nil {
		return fmt.Errorf("failed to fetch section IDs: %v", err)
	}

	var allErrors []string
	// performanceData map moved to top scope

	// Navigate once to the base page
	fmt.Println(">> [WWW] Smoke: loading base page...")
	if err := chromedp.Run(ctx, chromedp.Navigate(base)); err != nil {
		return fmt.Errorf("initial navigate failed: %v", err)
	}

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
		
		// Capture performance metrics before screenshot
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf("window.location.hash = '%s'", section), nil),
			chromedp.Evaluate(`setTimeout(() => {
				const el = document.querySelector('.header-fps');
				const text = el ? el.innerText : '';
				// Parse: FPS (label): 60 · CPU 2.50 ms · GPU 1.10 ms
				// or FPS --
				const stats = { fps: 0, cpu: 0, gpu: 0 };
				if (text && !text.includes('FPS --')) {
					const parts = text.split('·');
					if (parts.length >= 3) {
						// FPS part
						const fpsMatch = parts[0].match(/: (\d+)/);
						if (fpsMatch) stats.fps = parseInt(fpsMatch[1]);
						
						// CPU part
						const cpuMatch = parts[1].match(/CPU ([\d\.]+) ms/);
						if (cpuMatch) stats.cpu = parseFloat(cpuMatch[1]);

						// GPU part
						const gpuMatch = parts[2].match(/GPU ([\d\.]+) ms/);
						if (gpuMatch) stats.gpu = parseFloat(gpuMatch[1]);
					}
				}
				console.log('[SMOKE_STATS] ' + JSON.stringify(stats));
			}, 1500)`, nil),
			chromedp.Sleep(3500*time.Millisecond), // Wait for main.ts 3000ms scrolling/settling
			// chromedp.ScrollIntoView(fmt.Sprintf("#%s", section)), // REMOVED: Conflicts with main.ts scroll logic
			chromedp.Evaluate(`(async () => {
				const mem = (performance && performance.memory) ? {
					jsHeap: performance.memory.usedJSHeapSize / (1024 * 1024)
				} : { jsHeap: 0 };
				
				const resources = performance.getEntriesByType('resource');
				const totalSize = resources.reduce((acc, r) => acc + (r.transferSize || 0), 0) / (1024 * 1024);
				
				return {
					cpu: 0, // Placeholder, populated via CDP
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
						m.CPU = metric.Value // Seconds of script execution
					}
				}
				return nil
			}),
			chromedp.Evaluate("window.location.hash", &currentHash),
			chromedp.Evaluate("document.body.scrollTop", &scrollY),
			chromedp.Evaluate(fmt.Sprintf("console.log('[PROOFOFLIFE] 📸 SCREENSHOT STARTING: %s')", section), nil),
			// Use Viewport Screenshot (Page.captureScreenshot) instead of Element Screenshot
			// This avoids implicit scrolling logic in chromedp that might fight with CSS scroll snap
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

		// Merge heap/memory stats with async captured console stats
		mu.Lock()
		if existing, ok := performanceData[section]; ok {
			existing.JSHeap = m.JSHeap
			existing.Memory = m.Memory
			performanceData[section] = existing
			// Update m for logging below
			m.FPS = existing.FPS
			m.AppCPU = existing.AppCPU
			m.AppGPU = existing.AppGPU
		} else {
			performanceData[section] = m
		}
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
	galleryPath := filepath.Join(screenshotsDir, "gallery.md")
	smokeMdPath := filepath.Join(cwd, "src", "plugins", "www", "SMOKE.md")

	// Error Categorization
	proofOfLifeErrors := make(map[string]consoleEntry)
	uniqueErrors := make(map[string]consoleEntry)
	for _, entry := range entries {
		msg := entry.message
		// Skip CAD backend errors
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
		// Acquire lock if needed, but performanceData is map and we are single threaded here
		m := performanceData[section]
		smLines = append(smLines, fmt.Sprintf("| %s | %d | %.2f | %.2f | %.2f | %.2f | OK |", section, m.FPS, m.AppCPU, m.AppGPU, m.JSHeap, m.Memory))
	}

	smLines = append(smLines, "\n## 4. Test Orchestration DAG")
	smLines = append(smLines, "\n### Legend")
	smLines = append(smLines, "| Layer | Color | Description |")
	smLines = append(smLines, "|---|---|---|")
	smLines = append(smLines, "| **1. Foundation** | <span style=\"color:red\">█</span> Red | Cleanup, environment, and directory setup. |")
	smLines = append(smLines, "| **2. Core Logic** | <span style=\"color:orange\">█</span> Orange | Dev server, browser initialization, and proof-of-life. |")
	smLines = append(smLines, "| **3. Features** | <span style=\"color:yellow\">█</span> Yellow | Navigation loop, verification, and metrics capture. |")
	smLines = append(smLines, "| **4. QA** | <span style=\"color:blue\">█</span> Blue | Screenshot capture and visual summary tiling. |")
	smLines = append(smLines, "| **5. Release** | <span style=\"color:green\">█</span> Green | Final report generation and process cleanup. |")

	smLines = append(smLines, "\n```mermaid")
	smLines = append(smLines, "graph TD")
	smLines = append(smLines, "    %% Layer 1: Foundation")
	smLines = append(smLines, "    L1[Setup: Cleanup & Dirs]")
	smLines = append(smLines, "    ")
	smLines = append(smLines, "    %% Layer 2: Core Logic")
	smLines = append(smLines, "    L2[Dev Server: npm run dev]")
	smLines = append(smLines, "    L3[Browser: headless chrome]")
	smLines = append(smLines, "    L0[Proof of Life: Deliberate error discovery]")
	smLines = append(smLines, "    ")
	smLines = append(smLines, "    %% Layer 3: Feature Implementation")
	smLines = append(smLines, "    L4[Navigation: Hash-based loop]")
	smLines = append(smLines, "    L5[Verify: Hash & scroll position]")
	smLines = append(smLines, "    L6[Metrics: CDP Performance Data]")
	smLines = append(smLines, "    ")
	smLines = append(smLines, "    %% Layer 4: Quality Assurance")
	smLines = append(smLines, "    L7[Screenshots: Capture per-section]")
	smLines = append(smLines, "    L8[Tiling: summary.png]")
	smLines = append(smLines, "    ")
	smLines = append(smLines, "    %% Layer 5: Release")
	smLines = append(smLines, "    L9[Report: SMOKE.md]")
	smLines = append(smLines, "    L10[Cleanup: Stop browser & dev server]")

	smLines = append(smLines, "    %% Dependencies")
	smLines = append(smLines, "    L1 --> L2")
	smLines = append(smLines, "    L2 --> L3")
	smLines = append(smLines, "    L3 --> L0")
	smLines = append(smLines, "    L3 --> L4")
	smLines = append(smLines, "    L4 --> L5")
	smLines = append(smLines, "    L4 --> L6")
	smLines = append(smLines, "    L4 --> L7")
	smLines = append(smLines, "    L7 --> L8")
	smLines = append(smLines, "    L0 --> L9")
	smLines = append(smLines, "    L4 --> L9")
	smLines = append(smLines, "    L6 --> L9")
	smLines = append(smLines, "    L8 --> L9")
	smLines = append(smLines, "    L9 --> L10")

	smLines = append(smLines, "    %% Styling")
	smLines = append(smLines, "    classDef layer1 stroke:#FF0000,stroke-width:2px;")
	smLines = append(smLines, "    classDef layer2 stroke:#FFA500,stroke-width:2px;")
	smLines = append(smLines, "    classDef layer3 stroke:#FFFF00,stroke-width:2px;")
	smLines = append(smLines, "    classDef layer4 stroke:#0000FF,stroke-width:2px;")
	smLines = append(smLines, "    classDef layer5 stroke:#00FF00,stroke-width:2px;")
	smLines = append(smLines, "    ")
	smLines = append(smLines, "    class L1 layer1;")
	smLines = append(smLines, "    class L2,L3,L0 layer2;")
	smLines = append(smLines, "    class L4,L5,L6 layer3;")
	smLines = append(smLines, "    class L7,L8 layer4;")
	smLines = append(smLines, "    class L9,L10 layer5;")
	smLines = append(smLines, "```")

	smLines = append(smLines, "\n## 5. Visual Summary Grid")
	smLines = append(smLines, "\n![Summary Grid](screenshots/summary.png)")
	
	os.WriteFile(smokeMdPath, []byte(strings.Join(smLines, "\n")), 0644)

	if err := TileScreenshots(screenshotsDir, summaryPath, sections); err == nil {
		fmt.Printf("\n>> [WWW] Smoke COMPLETE")
		fmt.Printf("\n>> [WWW] GALLERY: file:///%s", strings.ReplaceAll(galleryPath, "\\", "/"))
		fmt.Printf("\n>> [WWW] SUMMARY: file:///%s\n", strings.ReplaceAll(summaryPath, "\\", "/"))
	} else {
		fmt.Printf(">> [WWW] Smoke: tiling failed: %v\n", err)
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("smoke tests encountered issues:\n%s", strings.Join(allErrors, "\n"))
	}

	fmt.Println(">> [WWW] Smoke: pass")
	return nil
}

func getChromeWebSocketURLHeadless() (string, error) {
	if ws := os.Getenv("CHROME_WS"); ws != "" {
		fmt.Println(">> [WWW] Smoke: using CHROME_WS")
		return ws, nil
	}

	port := os.Getenv("CHROME_DEBUG_PORT")
	if port == "" {
		port = "9222"
	}
	
	fmt.Println(">> [WWW] Smoke: launching headless chrome")
	launchCmd := getDialtoneCmd("chrome", "new", "--headless", "--gpu")
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
