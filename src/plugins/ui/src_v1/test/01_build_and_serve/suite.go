package buildandserve

import (
	"fmt"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	uitest "dialtone/dev/plugins/ui/src_v1/test"
)

var ctx = uitest.SharedContext()

type menuSectionCheck struct {
	NavAria      string
	SectionAria  string
	UnderlayAria string
}

var menuSmokeSections = []menuSectionCheck{
	{NavAria: "Open Home", SectionAria: "Docs Section", UnderlayAria: "Docs Underlay"},
	{NavAria: "Open Table", SectionAria: "Table Section", UnderlayAria: "Table Underlay"},
	{NavAria: "Open Three", SectionAria: "Three Section", UnderlayAria: "Three Underlay"},
	{NavAria: "Open Terminal", SectionAria: "Terminal Section", UnderlayAria: "Terminal Underlay"},
	{NavAria: "Open Camera", SectionAria: "Camera Section", UnderlayAria: "Camera Underlay"},
	{NavAria: "Open Home", SectionAria: "Docs Section", UnderlayAria: "Docs Underlay"},
}

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
	waitSection := 10 * time.Second
	waitClick := 5 * time.Second
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
	if err := sc.WaitForAriaLabel("Toggle Global Menu", waitSection); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("wait global menu toggle: %w", err)
	}
	for _, section := range menuSmokeSections {
		sc.Logf("MENU_NAV: visiting %s via %s", section.SectionAria, section.NavAria)
		if err := sc.ClickAriaLabelAfterWait("Toggle Global Menu", waitClick); err != nil {
			return testv1.StepRunResult{}, fmt.Errorf("open global menu for %s: %w", section.SectionAria, err)
		}
		if err := sc.ClickAriaLabelAfterWait(section.NavAria, waitClick); err != nil {
			return testv1.StepRunResult{}, fmt.Errorf("click %s: %w", section.NavAria, err)
		}
		if err := sc.WaitForAriaLabelAttrEquals(section.SectionAria, "data-active", "true", waitSection); err != nil {
			return testv1.StepRunResult{}, fmt.Errorf("wait active %s: %w", section.SectionAria, err)
		}
		if section.UnderlayAria != "" {
			if err := sc.WaitForAriaLabel(section.UnderlayAria, waitSection); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("wait underlay %s: %w", section.UnderlayAria, err)
			}
		}
	}
	if err := uitest.AssertJS(sc, 5*time.Second, `(() => {
		const s = document.getElementById('ui-home-docs');
		if (!s) return false;
		const h = s.querySelector('header');
		return !!h && h.classList.contains('shell-legend-text');
	})()`, "docs should use text legend mode"); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("assert docs legend mode: %w", err)
	}
	if err := uitest.CaptureScreenshot(sc, "ui_menu_sections.png"); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("capture screenshot ui_menu_sections.png: %w", err)
	}
	return testv1.StepRunResult{Report: fmt.Sprintf("fixture built, menu navigation visited home/table/three/terminal/camera and returned home, docs legend verified (attach=%t)", attach)}, nil
}
