package test

import (
	"context"
	"dialtone/cli/src/core/browser"
	"dialtone/cli/src/dialtest"
	"fmt"
	"math"
	"net"
	"time"

	"github.com/chromedp/chromedp"
)

func init() {
	dialtest.RegisterTicket("www-earth-camera-smooth")
	dialtest.AddSubtaskTest("init", func() error { return nil }, nil)
	dialtest.AddSubtaskTest("Verify-fix-with-automated-glitch-detection-test", RunGlitchTest, nil)
}

type Snapshot struct {
	Time     float64    `json:"time"`
	CamPos   [3]float64 `json:"camPos"`
	PoiIndex int        `json:"poiIndex"`
}

func RunGlitchTest() error {
	ctx, cancel, err := setupBrowser()
	if err != nil { return err }
	defer cancel()

	var snapshots []Snapshot
	
	// Navigate
	err = chromedp.Run(ctx,
		chromedp.Navigate("http://127.0.0.1:5174"),
		chromedp.Sleep(1*time.Second),
	)
	if err != nil { return err }

	fmt.Println("[GLITCH TEST] Starting high-frequency sampling (10 Hz) for 20 seconds...")
	
	start := time.Now()
	for time.Since(start) < 20*time.Second {
		var s Snapshot
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const d = window.earthDebug;
					if (!d) return null;
					return {
						time: performance.now(),
						camPos: [d.camera.position.x, d.camera.position.y, d.camera.position.z],
						poiIndex: d.currentPoiIndex
					};
				})()
			`, &s),
		)
		if err != nil { return err }
		if s.Time > 0 {
			snapshots = append(snapshots, s)
		}
		time.Sleep(100 * time.Millisecond) // 10 Hz
	}

	// Analyze for "jumps" (spikes in velocity)
	fmt.Printf("[GLITCH TEST] Captured %d snapshots. Analyzing for one-frame pops...\n", len(snapshots))
	
	glitchFound := false
	for i := 1; i < len(snapshots); i++ {
		p1, p2 := snapshots[i-1].CamPos, snapshots[i].CamPos
		dist := math.Sqrt(math.Pow(p2[0]-p1[0], 2) + math.Pow(p2[1]-p1[1], 2) + math.Pow(p2[2]-p1[2], 2))
		dt := (snapshots[i].Time - snapshots[i-1].Time) / 1000.0
		
		if dt <= 0 { continue }
		vel := dist / dt
		
		// A jump usually results in a massive velocity spike relative to the "gentle" 0.05 units/s
		if vel > 1.0 {
			fmt.Printf("[GLITCH DETECTED] Spike at T=%.2f: Velocity=%.4f units/s! (Dist=%.4f, Dt=%.4f)\n", 
				snapshots[i].Time/1000, vel, dist, dt)
			glitchFound = true
		}
	}

	if glitchFound {
		return fmt.Errorf("glitch detected: camera exhibited one-frame jumps or high-velocity pops")
	}

	fmt.Println("[GLITCH TEST] PASS: No high-velocity spikes detected over 20 seconds (including POI transition).")
	return nil
}

func setupBrowser() (context.Context, context.CancelFunc, error) {
	if !isPortOpen(5174) {
		return nil, nil, fmt.Errorf("dev server not found on port 5174")
	}

	if isPortOpen(9222) {
		allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), "http://127.0.0.1:9222")
		ctx, cancelCtx := chromedp.NewContext(allocCtx)
		return ctx, func() {
			cancelCtx()
			cancel()
		}, nil
	}

	execPath := browser.FindChromePath()
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Headless,
		chromedp.ExecPath(execPath),
		chromedp.Flag("remote-debugging-port", "9222"),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	
	return ctx, func() {
		cancelCtx()
		cancel()
	}, nil
}

func isPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	return false
}
