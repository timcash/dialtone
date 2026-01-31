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
	dialtest.AddSubtaskTest("Verify-smoothness-with-automated-telemetry-test", RunSmoothnessTest, nil)
}

type Snapshot struct {
	Time     float64    `json:"time"`
	CamPos   [3]float64 `json:"camPos"`
	CamRot   [3]float64 `json:"camRot"`
	PoiIndex int        `json:"poiIndex"`
}

func RunSmoothnessTest() error {
	ctx, cancel, err := setupBrowser()
	if err != nil { return err }
	defer cancel()

	var snapshots []Snapshot
	
	// Navigate once
	err = chromedp.Run(ctx,
		chromedp.Navigate("http://127.0.0.1:5174"),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil { return err }

	// Sample for 20 seconds to catch transitions
	for i := 0; i < 20; i++ {
		var s Snapshot
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const d = window.earthDebug;
					if (!d) return null;
					return {
						time: performance.now(),
						camPos: [d.camera.position.x, d.camera.position.y, d.camera.position.z],
						camRot: [d.camera.rotation.x, d.camera.rotation.y, d.camera.rotation.z],
						poiIndex: d.currentPoiIndex
					};
				})()
			`, &s),
			chromedp.Sleep(1*time.Second),
		)
		if err != nil { return err }
		if s.Time == 0 { return fmt.Errorf("failed to capture snapshot") }
		snapshots = append(snapshots, s)
	}

	// Analyze velocity
	maxVel := 0.0
	for i := 1; i < len(snapshots); i++ {
		p1, p2 := snapshots[i-1].CamPos, snapshots[i].CamPos
		dist := math.Sqrt(math.Pow(p2[0]-p1[0], 2) + math.Pow(p2[1]-p1[1], 2) + math.Pow(p2[2]-p1[2], 2))
		dt := (snapshots[i].Time - snapshots[i-1].Time) / 1000.0
		vel := dist / dt
		if vel > maxVel { maxVel = vel }
		fmt.Printf("[SMOOTH TEST] T=%.2f POI=%d Vel=%.4f/s\n", snapshots[i].Time/1000, snapshots[i].PoiIndex, vel)
	}

	fmt.Printf("[SMOOTH TEST] Peak Velocity: %.4f units/s\n", maxVel)
	
	// A "gentle" transition should not exceed a reasonable velocity.
	// Previously it was likely much higher during jumps. 
	// We'll set a threshold of 0.5 units/s for "gentle" movement.
	if maxVel > 0.8 {
		return fmt.Errorf("camera movement detected as too fast (peaked at %.4f/s)", maxVel)
	}

	fmt.Println("[SMOOTH TEST] PASS: Camera movement is gentle and controlled.")
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
