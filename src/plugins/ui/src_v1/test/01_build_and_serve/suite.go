package buildandserve

import (
	"fmt"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	uitest "dialtone/dev/plugins/ui/src_v1/test"
)

var ctx = uitest.SharedContext()

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:           "ui-build-and-go-serve",
		Timeout:        60 * time.Second,
		RunWithContext: runBuildAndServe,
	})
}

func runBuildAndServe(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	ctx.BeginStep(sc)
	defaultURL := ""
	if err := ctx.EnsureBuiltAndServed(); err != nil {
		return testv1.StepRunResult{}, err
	}
	defaultURL = ctx.AppURL("/#ui-home-docs")

	browserOpts, attach, err := uitest.BrowserOptionsFor(defaultURL)
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	navigateURL := strings.TrimSpace(browserOpts.URL)
	if navigateURL == "" {
		navigateURL = defaultURL
	}
	testv1.UpdateRuntimeConfig(func(cfg *testv1.RuntimeConfig) {
		cfg.BrowserNewTargetURL = navigateURL
	})
	if _, err := sc.EnsureBrowser(browserOpts); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("ensure browser: %w", err)
	}
	if err := sc.Goto(navigateURL); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("navigate attached browser: %w", err)
	}
	if err := uitest.SaveBrowserDebugConfig(sc); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("save browser debug config: %w", err)
	}
	if !attach {
		if err := uitest.ApplyMobileViewport(sc); err != nil {
			return testv1.StepRunResult{}, fmt.Errorf("apply mobile viewport: %w", err)
		}
	}
	// Attached sessions can recover onto a blank target after tab churn;
	// enforce a final navigate to the fixture URL before DOM assertions.
	if err := sc.Goto(navigateURL); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("re-navigate before hero assertions: %w", err)
	}
	if err := sc.WaitForAriaLabel("Docs Section", 10*time.Second); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("wait docs section: %w", err)
	}
	if err := sc.WaitForAriaLabelAttrEquals("Docs Section", "data-active", "true", 10*time.Second); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("wait docs section active attr: %w", err)
	}
	if err := sc.WaitForAriaLabel("Docs Underlay", 10*time.Second); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("wait docs underlay: %w", err)
	}
	if err := uitest.AssertJS(sc, 5*time.Second, `(() => {
		const s = document.getElementById('ui-home-docs');
		if (!s) return false;
		const h = s.querySelector('header');
		return !!h && h.classList.contains('shell-legend-text');
	})()`, "docs should use text legend mode"); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("assert docs legend mode: %w", err)
	}
	if err := uitest.CaptureScreenshot(sc, "ui_home_docs.png"); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("capture screenshot ui_home_docs.png: %w", err)
	}
	return testv1.StepRunResult{Report: fmt.Sprintf("fixture built, docs/home section loaded, text legend verified (attach=%t)", attach)}, nil
}
