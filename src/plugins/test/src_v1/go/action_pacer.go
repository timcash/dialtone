package test

import (
	"context"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

var (
	actionPacerMu       sync.Mutex
	actionPacerInterval time.Duration
	actionPacerNext     time.Time
)

// SetActionsPerMinute configures a global action pace for browser automation.
// A value <= 0 disables pacing.
func SetActionsPerMinute(apm float64) {
	actionPacerMu.Lock()
	defer actionPacerMu.Unlock()

	if apm <= 0 {
		actionPacerInterval = 0
		actionPacerNext = time.Time{}
		return
	}

	interval := time.Duration(float64(time.Minute) / apm)
	if interval < 0 {
		interval = 0
	}
	actionPacerInterval = interval
	actionPacerNext = time.Time{}
}

// SetClicksPerSecond is kept as a backward-compatible alias.
func SetClicksPerSecond(cps float64) {
	if cps <= 0 {
		SetActionsPerMinute(0)
		return
	}
	SetActionsPerMinute(cps * 60)
}

func paceAction(ctx context.Context) error {
	actionPacerMu.Lock()
	defer actionPacerMu.Unlock()

	if actionPacerInterval <= 0 {
		return nil
	}
	if !actionPacerNext.IsZero() {
		if wait := time.Until(actionPacerNext); wait > 0 {
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	actionPacerNext = time.Now().Add(actionPacerInterval)
	return nil
}

func runPacedChromedpAction(ctx context.Context, action chromedp.Action) error {
	if err := paceAction(ctx); err != nil {
		return err
	}
	return action.Do(ctx)
}
