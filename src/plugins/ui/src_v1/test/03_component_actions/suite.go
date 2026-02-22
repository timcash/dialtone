package componentactions

import (
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	uitest "dialtone/dev/plugins/ui/src_v1/test"
	"github.com/chromedp/chromedp"
)

var ctx = uitest.SharedContext()

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{Name: "ui-component-actions", Timeout: 45 * time.Second, RunWithContext: runComponents})
}

func runComponents(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	ctx.BeginStep(sc)
	if err := ctx.EnsureBuiltAndServed(); err != nil {
		return testv1.StepRunResult{}, err
	}
	if _, err := sc.EnsureBrowser(testv1.BrowserOptions{Headless: true, GPU: false, Role: "test", URL: ctx.AppURL("/#ui-meta-table")}); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabel("Table Section", 8*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Table Section", "data-active", "true", 8*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.WaitForBrowserMessageAfterAction("table-refreshed", 8*time.Second, func() error {
		return sc.ClickAriaLabelAfterWait("Table Thumb 1", 5*time.Second)
	}); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Table Status", "data-state", "refreshed", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.WaitForBrowserMessageAfterAction("three-add:1", 8*time.Second, func() error {
		if err := sc.RunBrowserWithTimeout(5*time.Second, chromedp.Evaluate(`window.navigateTo('ui-three-stage')`, nil)); err != nil {
			return err
		}
		if err := sc.WaitForAriaLabelAttrEquals("Three Section", "data-active", "true", 5*time.Second); err != nil {
			return err
		}
		return sc.ClickAriaLabelAfterWait("Three Add", 5*time.Second)
	}); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Three Count", "data-count", "1", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.WaitForBrowserMessageAfterAction("log-submit:ok", 8*time.Second, func() error {
		if err := sc.RunBrowserWithTimeout(5*time.Second, chromedp.Evaluate(`window.navigateTo('ui-log-xterm')`, nil)); err != nil {
			return err
		}
		if err := sc.WaitForAriaLabelAttrEquals("Log Section", "data-active", "true", 5*time.Second); err != nil {
			return err
		}
		if err := sc.TypeAriaLabel("Log Input", "ok"); err != nil {
			return err
		}
		return sc.PressEnterAriaLabel("Log Input")
	}); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Log Terminal", "data-last", "ok", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}

	ctx.Close()
	return testv1.StepRunResult{Report: "component interactions verified (table refresh, three add, log input enter)"}, nil
}
