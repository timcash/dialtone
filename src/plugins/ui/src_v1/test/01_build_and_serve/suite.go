package buildandserve

import (
	"fmt"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	uitest "dialtone/dev/plugins/ui/src_v1/test"
	sectionsnav "dialtone/dev/plugins/ui/src_v1/test/sections_navigation_lib"
)

type menuSectionCheck struct {
	NavAria      string
	SectionAria  string
	UnderlayAria string
}

var menuSmokeSections = []menuSectionCheck{
	{NavAria: "Open Table", SectionAria: "Table Section", UnderlayAria: "Table Underlay"},
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
	attach, err := sectionsnav.EnsureMenuBrowser(sc, true)
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	for _, section := range menuSmokeSections {
		if err := sectionsnav.OpenSectionFromMenu(sc, section.NavAria, section.SectionAria); err != nil {
			return testv1.StepRunResult{}, fmt.Errorf("open %s via menu: %w", section.SectionAria, err)
		}
		if section.UnderlayAria != "" {
			if err := sc.WaitForAriaLabel(section.UnderlayAria, sectionsnav.WaitSection); err != nil {
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
	return testv1.StepRunResult{Report: fmt.Sprintf("fixture built, served, and smoke-verified through menu navigation table -> home with docs legend intact (attach=%t)", attach)}, nil
}
