package test

import (
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

func ApplyMobileViewport(sc *StepContext) error {
	return sc.RunBrowserWithTimeout(4*time.Second, chromedp.Tasks{
		chromedp.EmulateViewport(393, 852, chromedp.EmulateScale(3)),
		emulation.SetDeviceMetricsOverride(393, 852, 3, true),
		emulation.SetTouchEmulationEnabled(true),
	})
}
