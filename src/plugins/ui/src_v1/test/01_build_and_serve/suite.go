package buildandserve

import (
	"fmt"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	uitest "dialtone/dev/plugins/ui/src_v1/test"
)

var ctx = uitest.SharedContext()

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{Name: "ui-build-and-go-serve", Timeout: 60 * time.Second, RunWithContext: runBuildAndServe})
}

func runBuildAndServe(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	ctx.BeginStep(sc)
	defaultURL := ""
	if err := ctx.EnsureBuiltAndServed(); err != nil {
		return testv1.StepRunResult{}, err
	}
	defaultURL = ctx.AppURL("/#ui-hero-stage")

	browserOpts, attach, err := uitest.BrowserOptionsFor(defaultURL)
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	if _, err := sc.EnsureBrowser(browserOpts); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("ensure browser: %w", err)
	}
	if err := sc.WaitForAriaLabel("App Header", 10*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabel("Hero Section", 10*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Hero Section", "data-ready", "true", 10*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	return testv1.StepRunResult{Report: fmt.Sprintf("fixture UI loaded and hero ready (attach=%t)", attach)}, nil
}
