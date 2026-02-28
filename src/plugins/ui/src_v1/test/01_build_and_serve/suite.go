package buildandserve

import (
	"fmt"
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
	defaultURL = ctx.AppURL("/#hero")
	testv1.UpdateRuntimeConfig(func(cfg *testv1.RuntimeConfig) {
		cfg.BrowserNewTargetURL = defaultURL
	})

	browserOpts, attach, err := uitest.BrowserOptionsFor(defaultURL)
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	if _, err := sc.EnsureBrowser(browserOpts); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("ensure browser: %w", err)
	}
	if err := sc.RunBrowserWithTimeout(8*time.Second, testv1.Navigate(defaultURL)); err != nil {
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
	if err := sc.RunBrowserWithTimeout(8*time.Second, testv1.Navigate(defaultURL)); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("re-navigate before hero assertions: %w", err)
	}
	if err := sc.WaitForAriaLabel("Hero Section", 10*time.Second); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("wait hero section: %w", err)
	}
	if err := sc.WaitForAriaLabelAttrEquals("Hero Section", "data-active", "true", 10*time.Second); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("wait hero section active attr: %w", err)
	}
	if err := sc.WaitForAriaLabel("Hero Canvas", 10*time.Second); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("wait hero canvas: %w", err)
	}
	if err := uitest.AssertJS(sc, 5*time.Second, `(() => {
		const s = document.getElementById('hero');
		if (!s) return false;
		const h = s.querySelector('header');
		return !!h && h.classList.contains('legend');
	})()`, "hero should use legend header mode"); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("assert hero legend mode: %w", err)
	}
	if err := uitest.CaptureScreenshot(sc, "ui_hero.png"); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("capture screenshot ui_hero.png: %w", err)
	}
	return testv1.StepRunResult{Report: fmt.Sprintf("fixture built, hero section loaded, legend header verified (attach=%t)", attach)}, nil
}
