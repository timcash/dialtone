package componentactions

import (
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	uitest "dialtone/dev/plugins/ui/src_v1/test"
	"github.com/chromedp/chromedp"
)

var ctx = uitest.SharedContext()

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:    "ui-component-actions-and-modes",
		Timeout: 75 * time.Second,
		Screenshots: []string{
			"plugins/ui/src_v1/test/screenshots/ui_table_fullscreen.png",
			"plugins/ui/src_v1/test/screenshots/ui_terminal.png",
			"plugins/ui/src_v1/test/screenshots/ui_three_calculator.png",
		},
		RunWithContext: runComponents,
	})
}

func runComponents(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	ctx.BeginStep(sc)
	if err := ctx.EnsureBuiltAndServed(); err != nil {
		return testv1.StepRunResult{}, err
	}
	browserOpts, attach, err := uitest.BrowserOptionsFor(ctx.AppURL("/#table"))
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	if _, err := sc.EnsureBrowser(browserOpts); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := uitest.ApplyMobileViewport(sc); err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.WaitForAriaLabelAttrEquals("Table Section", "data-active", "true", 8*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForBrowserMessageAfterAction("table-refreshed", 8*time.Second, func() error {
		return sc.ClickAriaLabelAfterWait("Table Refresh", 5*time.Second)
	}); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := uitest.AssertJS(sc, 5*time.Second, `(() => {
		const s = document.getElementById('table');
		return !!s && s.classList.contains('calculator');
	})()`, "table should start in calculator mode"); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.ClickAriaLabelAfterWait("Table Mode", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := uitest.AssertJS(sc, 5*time.Second, `(() => {
		const s = document.getElementById('table');
		return !!s && s.classList.contains('fullscreen');
	})()`, "table should switch to fullscreen mode"); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := uitest.CaptureScreenshot(sc, "ui_table_fullscreen.png"); err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.ClickAriaLabelAfterWait("Toggle Global Menu", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.ClickAriaLabelAfterWait("Navigate Terminal", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Terminal Section", "data-active", "true", 8*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForBrowserMessageAfterAction("log-submit:ok", 8*time.Second, func() error {
		if err := sc.TypeAriaLabel("Terminal Input", "ok"); err != nil {
			return err
		}
		return sc.ClickAriaLabel("Terminal Send")
	}); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := uitest.CaptureScreenshot(sc, "ui_terminal.png"); err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.ClickAriaLabelAfterWait("Toggle Global Menu", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.ClickAriaLabelAfterWait("Navigate Three Calculator", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Three Calculator Section", "data-active", "true", 8*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForBrowserMessageAfterAction("three-add:1", 8*time.Second, func() error {
		return sc.ClickAriaLabelAfterWait("Three Add", 5*time.Second)
	}); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.RunBrowserWithTimeout(5*time.Second, chromedp.Evaluate(`(() => {
		const panel = document.querySelector("nav [aria-label='Global Menu Panel']");
		if (!panel) return false;
		return panel.hasAttribute('hidden');
	})()`, nil)); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := uitest.CaptureScreenshot(sc, "ui_three_calculator.png"); err != nil {
		return testv1.StepRunResult{}, err
	}

	if !attach {
		ctx.Close()
	}
	return testv1.StepRunResult{Report: "component actions verified (mode toggle, table refresh, terminal send, three add) with mobile screenshots"}, nil
}
