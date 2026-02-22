package exampleplugintemplate

import (
	"fmt"
	"time"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/chromedp/chromedp"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name: "example-template-step",
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			if err := sc.WaitForStepMessageAfterAction("template plugin info", 4*time.Second, func() error {
				sc.Infof("template plugin info")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := sc.WaitForStepMessageAfterAction("template plugin error", 4*time.Second, func() error {
				sc.Errorf("template plugin error")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "template-style test step ran in shared process"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "example-browser-stepcontext-api",
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			if chrome.FindChromePath() == "" {
				sc.Warnf("chrome not found; skipping browser helper example")
				return testv1.StepRunResult{Report: "skipped browser helper example (chrome not installed)"}, nil
			}
			b, err := sc.EnsureBrowser(testv1.BrowserOptions{
				Headless:      true,
				GPU:           false,
				Role:          "test",
				ReuseExisting: false,
				URL:           "data:text/html,<button aria-label='Do Thing' onclick=\"console.log('clicked')\">Go</button>",
			})
			if err != nil {
				sc.Warnf("browser not available for template demo: %v", err)
				return testv1.StepRunResult{Report: "skipped browser helper example (browser not available)"}, nil
			}

			if err := sc.WaitForAriaLabel("Do Thing", 10*time.Second); err != nil {
				sc.Warnf("browser aria wait failed: %v", err)
				return testv1.StepRunResult{Report: "skipped browser helper example (aria wait failed)"}, nil
			}
			if err := sc.ClickAriaLabel("Do Thing"); err != nil {
				sc.Warnf("browser click failed: %v", err)
				return testv1.StepRunResult{Report: "skipped browser helper example (click failed)"}, nil
			}
			if err := sc.WaitForConsoleContains("clicked", 5*time.Second); err != nil {
				sc.Warnf("browser console wait failed: %v", err)
				return testv1.StepRunResult{Report: "skipped browser helper example (console wait failed)"}, nil
			}
			var title string
			if err := b.Run(chromedp.Evaluate(`document.title || ''`, &title)); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("eval title: %w", err)
			}
			return testv1.StepRunResult{Report: "StepContext browser helpers ready (aria + console + chromedp)"}, nil
		},
	})
}
