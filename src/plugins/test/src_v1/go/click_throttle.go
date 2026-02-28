package test

import (
	"context"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

var (
	clickThrottleMu       sync.Mutex
	clickThrottleInterval time.Duration
	clickThrottleNext     time.Time
)

// SetClicksPerSecond configures a global click/tap throttle for test actions.
// A value <= 0 disables throttling.
func SetClicksPerSecond(cps float64) {
	clickThrottleMu.Lock()
	defer clickThrottleMu.Unlock()

	if cps <= 0 {
		clickThrottleInterval = 0
		clickThrottleNext = time.Time{}
		return
	}

	interval := time.Duration(float64(time.Second) / cps)
	if interval < 0 {
		interval = 0
	}
	clickThrottleInterval = interval
	clickThrottleNext = time.Time{}
}

// runThrottledClick serializes all UI click/tap actions and enforces the
// configured interval between completed click actions across the whole test run.
func runThrottledClick(ctx context.Context, click chromedp.Action) error {
	clickThrottleMu.Lock()
	defer clickThrottleMu.Unlock()

	if clickThrottleInterval <= 0 {
		return click.Do(ctx)
	}
	if !clickThrottleNext.IsZero() {
		if wait := time.Until(clickThrottleNext); wait > 0 {
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	if err := click.Do(ctx); err != nil {
		return err
	}
	clickThrottleNext = time.Now().Add(clickThrottleInterval)
	return nil
}
