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
	dialtest.RegisterTicket("www-earth-cloud-motion")
	dialtest.AddSubtaskTest("init", func() error { return nil }, nil)
	dialtest.AddSubtaskTest("Verify-cloud-motion-speed", RunCloudMotionTest, nil)
}

type CloudSnapshot struct {
	Time float64 `json:"time"`
	RotY float64 `json:"rotY"`
}

func RunCloudMotionTest() error {
	ctx, cancel, err := setupBrowser()
	if err != nil {
		return err
	}
	defer cancel()

	// Navigate to local dev server (port 5176 based on previous output)
	targetURL := "http://127.0.0.1:5176"
	if !isPortOpen(5176) {
		if isPortOpen(5175) {
			targetURL = "http://127.0.0.1:5175"
		} else if isPortOpen(5174) {
			targetURL = "http://127.0.0.1:5174"
		}
	}

	err = chromedp.Run(ctx,
		chromedp.Navigate(targetURL),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		return err
	}

	fmt.Printf("[CLOUD TEST] Sampling cloud rotation at %s...\n", targetURL)

	var initial, final CloudSnapshot

	// Sample 1
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const d = window.earthDebug;
				if (!d || !d.cloud1) return null;
				return { time: performance.now(), rotY: d.cloud1.rotation.y };
			})()
		`, &initial),
	)
	if err != nil {
		return err
	}
	if initial.Time == 0 {
		return fmt.Errorf("could not access cloud rotation data")
	}

	time.Sleep(5 * time.Second)

	// Sample 2
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const d = window.earthDebug;
				if (!d || !d.cloud1) return null;
				return { time: performance.now(), rotY: d.cloud1.rotation.y };
			})()
		`, &final),
	)
	if err != nil {
		return err
	}

	deltaRot := MathAbs(final.RotY - initial.RotY)
	deltaTime := (final.Time - initial.Time) / 1000.0
	rotPerSec := deltaRot / deltaTime

	fmt.Printf("[CLOUD TEST] DeltaRot: %.6f, DeltaTime: %.2fs, Speed: %.6f rad/s\n", deltaRot, deltaTime, rotPerSec)

	// We increased cloud1RotSpeed to 0.00025.
	// The test should verify it's at least 80% of that value (accounting for rendering/timing jitter).
	minSpeed := 0.00020
	if rotPerSec < minSpeed {
		return fmt.Errorf("FAIL: Cloud rotation too slow! Expected > %.6f, got %.6f", minSpeed, rotPerSec)
	}

	fmt.Println("[CLOUD TEST] PASS: Cloud rotation speed verified.")
	return nil
}

func MathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func setupBrowser() (context.Context, context.CancelFunc, error) {
	browser.CleanupPort(9222)
	execPath := browser.FindChromePath()
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Headless,
		chromedp.ExecPath(execPath),
		chromedp.Flag("remote-debugging-port", "9222"),
		chromedp.Flag("no-sandbox", true),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	return ctx, func() { cancelCtx(); cancel() }, nil
}

func isPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	return false
}
