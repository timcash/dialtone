package test

import (
	"context"
	"dialtone/cli/src/core/browser"
	"dialtone/cli/src/dialtest"
	"fmt"
	"net"
	"time"

	"github.com/chromedp/chromedp"
)

func init() {
	dialtest.RegisterTicket("www-earth-refactor")
	dialtest.AddSubtaskTest("init", func() error { return nil }, nil)
	dialtest.AddSubtaskTest("Verify-focus-stability-at-high-altitude", RunFocusTest, nil)
}

type FocusSnapshot struct {
	Time         float64    `json:"time"`
	CamDist      float64    `json:"camDist"`
	HorizonAngle float64    `json:"horizonAngle"`
	LookAngle    float64    `json:"lookAngle"`
}

func RunFocusTest() error {
	ctx, cancel, err := setupBrowser()
	if err != nil { return err }
	defer cancel() // Ensure cleanup

	var snapshots []FocusSnapshot
	
	// Navigate
	err = chromedp.Run(ctx,
		chromedp.Navigate("http://127.0.0.1:5174"),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil { return err }

	fmt.Println("[FOCUS TEST] Sampling camera gaze for 30 seconds...")
	
	start := time.Now()
	for time.Since(start) < 30*time.Second {
		var s FocusSnapshot
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const d = window.earthDebug;
					if (!d) return null;
					const cam = d.camera;
					const camWorld = cam.position;
					const camDist = camWorld.length();
					const earthRadius = d.earthRadius; 
					const horizonAngle = Math.asin(Math.min(0.99, earthRadius / camDist));
					
					const toEarthCenter = camWorld.clone().negate().normalize();
					const lookDir = new THREE.Vector3(0, 0, -1).applyQuaternion(cam.quaternion);
					const lookAngle = Math.acos(Math.min(1.0, lookDir.dot(toEarthCenter)));

					return {
						time: performance.now(),
						camDist: camDist,
						horizonAngle: horizonAngle,
						lookAngle: lookAngle
					};
				})()
			`, &s),
		)
		if err != nil { return err }
		if s.Time > 0 {
			snapshots = append(snapshots, s)
			// Check immediately
			if s.LookAngle > s.HorizonAngle * 1.01 {
				return fmt.Errorf("FAIL: Camera looking into space! LookAngle (%.4f) > HorizonAngle (%.4f) at T=%.2f", 
					s.LookAngle, s.HorizonAngle, s.Time/1000)
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("[FOCUS TEST] PASS: Camera gaze remained within Earth's silhouette.")
	return nil
}

func setupBrowser() (context.Context, context.CancelFunc, error) {
	if !isPortOpen(5174) {
		return nil, nil, fmt.Errorf("dev server not found on port 5174")
	}

	browser.CleanupPort(9222) // Kill stale browser

	execPath := browser.FindChromePath()
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Headless,
		chromedp.ExecPath(execPath),
		chromedp.Flag("remote-debugging-port", "9222"),
		chromedp.Flag("remote-debugging-address", "127.0.0.1"),
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
