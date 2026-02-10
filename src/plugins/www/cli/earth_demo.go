package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"github.com/chromedp/cdproto/performance"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func handleEarthDemo(webDir string) {
	logInfo("Setting up Earth Demo Environment...")

	// 1. Aggressive Port Cleanup
	logInfo("Cleaning up port 5173...")
	_ = exec.Command("fuser", "-k", "5173/tcp").Run()
	time.Sleep(1500 * time.Millisecond)

	// 2. Kill existing Dialtone Chrome instances
	logInfo("Cleaning up Chrome processes...")
	_ = exec.Command("./dialtone.sh", "chrome", "kill", "all").Run()

	// 3. Start WWW Dev Server (Background)
	logInfo("Starting WWW Dev Server...")
	devCmd := exec.Command("npm", "run", "dev", "--", "--host", "127.0.0.1")
	devCmd.Dir = webDir

	stdout, err := devCmd.StdoutPipe()
	if err != nil {
		logFatal("Failed to attach to dev server stdout: %v", err)
	}
	stderr, err := devCmd.StderrPipe()
	if err != nil {
		logFatal("Failed to attach to dev server stderr: %v", err)
	}

	if err := devCmd.Start(); err != nil {
		logFatal("Failed to start dev server: %v", err)
	}

	// 4. Wait for dev server to be ready + detect actual port
	logInfo("Waiting for Dev Server...")
	port := 5173
	portCh := make(chan int, 1)

	go func() {
		reader := io.MultiReader(stdout, stderr)
		scanner := bufio.NewScanner(reader)
		re := regexp.MustCompile(`http://127\.0\.0\.1:(\d+)/`)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
			if match := re.FindStringSubmatch(line); len(match) == 2 {
				if p, err := strconv.Atoi(match[1]); err == nil {
					select {
					case portCh <- p:
					default:
					}
				}
			}
		}
	}()

	select {
	case detected := <-portCh:
		port = detected
	case <-time.After(10 * time.Second):
		logInfo("Dev server port not detected yet; falling back to %d", port)
	}

	ready := false
	for i := 0; i < 30; i++ {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !ready {
		logFatal("Dev server failed to start within 30 seconds")
	}

	// 5. Launch GPU-enabled Chrome on Earth section
	logInfo("Launching GPU-enabled Chrome...")
	baseURL := fmt.Sprintf("http://127.0.0.1:%d/#s-home", port)
	chromeCmd := exec.Command("./dialtone.sh", "chrome", "new", baseURL, "--gpu")
	output, err := chromeCmd.CombinedOutput()
	if err != nil {
		logFatal("Failed to launch Chrome: %v\nOutput: %s", err, string(output))
	}

	re := regexp.MustCompile(`ws://127\.0\.0\.1:\d+/devtools/browser/[a-z0-9-]+`)
	wsURL := re.FindString(string(output))
	if wsURL == "" {
		logFatal("Failed to find WebSocket URL in chrome output: %s", string(output))
	}

	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	var consoleLogs []string
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			for _, arg := range ev.Args {
				consoleLogs = append(consoleLogs, fmt.Sprintf("[%s] %s", ev.Type, arg.Value))
			}
		case *runtime.EventExceptionThrown:
			consoleLogs = append(consoleLogs, fmt.Sprintf("[EXCEPTION] %s", ev.ExceptionDetails.Text))
		}
	})

	logInfo("Capturing Earth performance metrics via Chrome DevTools...")
	var framesStart struct {
		Frames int `json:"frames"`
	}
	var framesEnd struct {
		Frames int `json:"frames"`
	}
	var domCheck struct {
		HasEarth  bool `json:"hasEarth"`
		HasCanvas bool `json:"hasCanvas"`
		HasDebug  bool `json:"hasDebug"`
	}
	var rendererInfo struct {
		Render   map[string]any `json:"render"`
		Memory   map[string]any `json:"memory"`
		Programs int            `json:"programs"`
	}
	var glInfo struct {
		Vendor   string `json:"vendor"`
		Renderer string `json:"renderer"`
		Version  string `json:"version"`
	}

	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL),
		chromedp.Evaluate(`(function(){
			const el = document.getElementById("s-home");
			if (el) el.scrollIntoView({ behavior: "instant" });
			return true;
		})()`, nil),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`(function(){
			const earth = document.getElementById("earth-container");
			const canvas = earth ? earth.querySelector("canvas") : null;
			return {
				hasEarth: !!earth,
				hasCanvas: !!canvas,
				hasDebug: !!window.earthDebug,
			};
		})()`, &domCheck),
		chromedp.Evaluate(`(function(){
			const start = window.earthDebug ? window.earthDebug.frameCount : 0;
			return { frames: start };
		})()`, &framesStart),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`(function(){
			const end = window.earthDebug ? window.earthDebug.frameCount : 0;
			return { frames: end };
		})()`, &framesEnd),
		chromedp.Evaluate(`(function(){
			const info = window.earthDebug?.renderer?.info;
			if (!info) return null;
			return {
				render: info.render,
				memory: info.memory,
				programs: info.programs ? info.programs.length : 0,
			};
		})()`, &rendererInfo),
		chromedp.Evaluate(`(function(){
			const canvas = document.querySelector("#earth-container canvas");
			if (!canvas) return null;
			const gl = canvas.getContext("webgl2") || canvas.getContext("webgl");
			if (!gl) return null;
			const dbg = gl.getExtension("WEBGL_debug_renderer_info");
			return {
				vendor: dbg ? gl.getParameter(dbg.UNMASKED_VENDOR_WEBGL) : gl.getParameter(gl.VENDOR),
				renderer: dbg ? gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL) : gl.getParameter(gl.RENDERER),
				version: gl.getParameter(gl.VERSION),
			};
		})()`, &glInfo),
	)

	if err != nil {
		logInfo("Chromedp run failed: %v", err)
	}

	logInfo("DOM check: earthContainer=%v canvas=%v earthDebug=%v",
		domCheck.HasEarth,
		domCheck.HasCanvas,
		domCheck.HasDebug,
	)

	metrics, err := performance.GetMetrics().Do(ctx)
	if err == nil {
		logInfo("Performance metrics:")
		for _, m := range metrics {
			switch m.Name {
			case "Frames", "JSHeapUsedSize", "Nodes", "TaskDuration", "ScriptDuration", "LayoutDuration":
				logInfo("  %s: %.2f", m.Name, m.Value)
			}
		}
	}

	fps := float64(framesEnd.Frames-framesStart.Frames) / 3.0
	logInfo("Earth FPS (sampled): %.1f", fps)
	logInfo("Renderer info: drawCalls=%v triangles=%v lines=%v points=%v",
		rendererInfo.Render["calls"],
		rendererInfo.Render["triangles"],
		rendererInfo.Render["lines"],
		rendererInfo.Render["points"],
	)
	logInfo("Memory info: geometries=%v textures=%v",
		rendererInfo.Memory["geometries"],
		rendererInfo.Memory["textures"],
	)
	logInfo("Shader programs: %d", rendererInfo.Programs)
	if glInfo.Renderer != "" {
		logInfo("WebGL renderer: %s | vendor: %s | version: %s", glInfo.Renderer, glInfo.Vendor, glInfo.Version)
	}
	if len(consoleLogs) > 0 {
		logInfo("Browser console logs:")
		for _, line := range consoleLogs {
			fmt.Printf("  %s\n", line)
		}
	}

	logInfo("Earth Demo Environment is LIVE!")
	logInfo("Dev Server: %s", baseURL)
	logInfo("Press Ctrl+C to stop...")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	logInfo("Shutting down...")
}
