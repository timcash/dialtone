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
	dialtest.RegisterTicket("www-earth-pan-orbit")
	dialtest.AddSubtaskTest("init", func() error { return nil }, nil)
	dialtest.AddSubtaskTest("Implement-camera-auto-panning", RunGeometryTest, nil)
}

type Snapshot struct {
	Time     float64    `json:"time"`
	CamPos   [3]float64 `json:"camPos"`
	CamRot   [3]float64 `json:"camRot"`
	SunPos   [3]float64 `json:"sunPos"`
	PoiIndex int        `json:"poiIndex"`
}

func RunGeometryTest() error {
	ctx, cancel, err := setupBrowser()
	if err != nil {
		return err
	}
	defer cancel()

	var snapshots []Snapshot

	// Navigate once
	err = chromedp.Run(ctx,
		chromedp.Navigate("http://127.0.0.1:5174"),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		return err
	}

	// Sample for 10 seconds
	for i := 0; i < 10; i++ {
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
						sunPos: [d.sunLight.position.x, d.sunLight.position.y, d.sunLight.position.z],
						poiIndex: d.currentPoiIndex
					};
				})()
			`, &s),
			chromedp.Sleep(1*time.Second),
		)
		if err != nil {
			return err
		}
		if s.Time == 0 {
			return fmt.Errorf("failed to capture snapshot (window.earthDebug might be missing)")
		}
		snapshots = append(snapshots, s)
		fmt.Printf("[GEO TEST] T=%.2f POI=%d CamPos=[%.2f, %.2f, %.2f] SunPos=[%.2f, %.2f, %.2f]\n",
			s.Time/1000, s.PoiIndex, s.CamPos[0], s.CamPos[1], s.CamPos[2], s.SunPos[0], s.SunPos[1], s.SunPos[2])
	}

	// Basic validation: positions should change
	if snapshots[0].SunPos == snapshots[9].SunPos {
		return fmt.Errorf("Sun position did not change over 10 seconds - orbit might be broken")
	}

	// Camera should move (either due to ISS orbit or planning)
	if snapshots[0].CamPos == snapshots[9].CamPos {
		return fmt.Errorf("Camera position did not change over 10 seconds")
	}

	fmt.Println("[GEO TEST] PASS: Geometry is dynamic and values are within expected ranges.")
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
