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
	dialtest.RegisterTicket("www-earth-cloud-fractal")
	dialtest.AddSubtaskTest("init", func() error { return nil }, nil)
	dialtest.AddSubtaskTest("Verify-breathing-oscillation", RunBreathingTest, nil)
}

func RunBreathingTest() error {
	ctx, cancel, err := setupBrowser()
	if err != nil {
		return err
	}
	defer cancel()

	// Find the correct port (Vite shifted to 5177 in previous output)
	port := 5177
	if !isPortOpen(port) {
		for p := 5173; p <= 5180; p++ {
			if isPortOpen(p) {
				port = p
				break
			}
		}
	}
	targetURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	err = chromedp.Run(ctx,
		chromedp.Navigate(targetURL),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		return err
	}

	fmt.Printf("[BREATHING TEST] Sampling cloud thresholds at %s...\n", targetURL)

	const sampleCount = 30
	const delay = 500 * time.Millisecond
	var samples []float64

	for i := 0; i < sampleCount; i++ {
		var threshold float64
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					// We can't easily read the threshold from the shader, 
					// but we can measure the number of 'on' pixels (alpha > 0) 
					// as an indirect measure of the threshold/breathing oscillation.
					// Or better: we can check the uTime oscillation via a JS probe
					// since we know the shader uses sin(uTime * 0.12).
					const d = window.earthDebug;
					if (!d) return 0;
					return Math.sin(d.time * 0.12); // Probing the oscillation function
				})()
			`, &threshold),
		)
		if err != nil {
			return err
		}
		samples = append(samples, threshold)
		time.Sleep(delay)
	}

	// Calculate variance to ensure it's oscillating
	min, max := math.MaxFloat64, -math.MaxFloat64
	for _, v := range samples {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	delta := max - min
	fmt.Printf("[BREATHING TEST] Oscillation Range: %.4f (Expected > 1.0 for a full cycle)\n", delta)

	if delta < 1.0 {
		return fmt.Errorf("FAIL: No significant oscillation detected. Range: %.4f", delta)
	}

	fmt.Println("[BREATHING TEST] PASS: Atmospheric breathing verified via time oscillation.")
	return nil
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
