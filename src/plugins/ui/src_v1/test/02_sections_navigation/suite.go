package sectionsnavigation

import (
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	uitest "dialtone/dev/plugins/ui/src_v1/test"
)

var ctx = uitest.SharedContext()

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{Name: "ui-section-navigation-via-menu", Timeout: 45 * time.Second, RunWithContext: runNavigation})
}

func runNavigation(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	ctx.BeginStep(sc)
	if err := ctx.EnsureBuiltAndServed(); err != nil {
		return testv1.StepRunResult{}, err
	}
	if _, err := sc.EnsureBrowser(testv1.BrowserOptions{Headless: true, GPU: false, Role: "test", URL: ctx.AppURL("/#ui-hero-stage")}); err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.WaitForAriaLabel("Toggle Global Menu", 8*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.ClickAriaLabelAfterWait("Toggle Global Menu", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.ClickAriaLabelAfterWait("Navigate Docs", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Docs Section", "data-active", "true", 8*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.ClickAriaLabelAfterWait("Toggle Global Menu", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.ClickAriaLabelAfterWait("Navigate Table", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Table Section", "data-active", "true", 8*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.ClickAriaLabelAfterWait("Toggle Global Menu", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.ClickAriaLabelAfterWait("Navigate Stage", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Three Section", "data-active", "true", 8*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}

	return testv1.StepRunResult{Report: "menu + section navigation verified (hero/docs/table/stage)"}, nil
}
