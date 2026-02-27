package test

import (
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

func AssertJS(sc *StepContext, timeout time.Duration, expr string, failMsg string) error {
	var ok bool
	if err := sc.RunBrowserWithTimeout(timeout, chromedp.Evaluate(expr, &ok)); err != nil {
		return err
	}
	if !ok {
		if failMsg == "" {
			failMsg = "javascript assertion failed"
		}
		return fmt.Errorf("%s", failMsg)
	}
	return nil
}
