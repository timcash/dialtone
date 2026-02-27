package sectionsnavigation

import (
	"fmt"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	uitest "dialtone/dev/plugins/ui/src_v1/test"
)

var ctx = uitest.SharedContext()

type sectionCase struct {
	id           string
	navAria      string
	sectionAria  string
	screenshot   string
	assertJSExpr string
	assertFail   string
}

var sectionCases = []sectionCase{
	{
		id:          "docs",
		navAria:     "Navigate Docs",
		sectionAria: "Docs Section",
		screenshot:  "ui_docs.png",
		assertJSExpr: `(() => {
			const s = document.getElementById('docs');
			return !!s && s.classList.contains('fullscreen') && !!s.querySelector('header.text');
		})()`,
		assertFail: "docs should be fullscreen with text header",
	},
	{
		id:          "table",
		navAria:     "Navigate Table",
		sectionAria: "Table Section",
		screenshot:  "ui_table.png",
		assertJSExpr: `(() => {
			const s = document.getElementById('table');
			return !!s && s.classList.contains('calculator') && !!s.querySelector('header.legend');
		})()`,
		assertFail: "table should be calculator with legend header",
	},
	{
		id:           "three-fullscreen",
		navAria:      "Navigate Three Fullscreen",
		sectionAria:  "Three Fullscreen Section",
		screenshot:   "ui_three_fullscreen.png",
		assertJSExpr: "",
		assertFail:   "",
	},
	{
		id:           "camera",
		navAria:      "Navigate Camera",
		sectionAria:  "Camera Section",
		screenshot:   "ui_camera.png",
		assertJSExpr: "",
		assertFail:   "",
	},
	{
		id:          "settings",
		navAria:     "Navigate Settings",
		sectionAria: "Settings Section",
		screenshot:  "ui_settings.png",
		assertJSExpr: `(() => {
			const s = document.getElementById('settings');
			return !!s && s.classList.contains('fullscreen') && !!s.querySelector('header.text');
		})()`,
		assertFail: "settings should be fullscreen with text header",
	},
	{
		id:           "terminal",
		navAria:      "Navigate Terminal",
		sectionAria:  "Terminal Section",
		screenshot:   "ui_terminal_section.png",
		assertJSExpr: "",
		assertFail:   "",
	},
	{
		id:           "three-calculator",
		navAria:      "Navigate Three Calculator",
		sectionAria:  "Three Calculator Section",
		screenshot:   "ui_three_calculator_section.png",
		assertJSExpr: "",
		assertFail:   "",
	},
}

func Register(reg *testv1.Registry) {
	for _, c := range sectionCases {
		tc := c
		reg.Add(testv1.Step{
			Name:        fmt.Sprintf("ui-section-%s-via-menu", tc.id),
			Timeout:     40 * time.Second,
			Screenshots: []string{fmt.Sprintf("plugins/ui/src_v1/test/screenshots/%s", tc.screenshot)},
			RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
				return runSection(sc, tc)
			},
		})
	}
}

func runSection(sc *testv1.StepContext, c sectionCase) (testv1.StepRunResult, error) {
	ctx.BeginStep(sc)
	if err := ctx.EnsureBuiltAndServed(); err != nil {
		return testv1.StepRunResult{}, err
	}
	browserOpts, _, err := uitest.BrowserOptionsFor(ctx.AppURL("/#hero"))
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	if _, err := sc.EnsureBrowser(browserOpts); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := uitest.ApplyMobileViewport(sc); err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.WaitForAriaLabel("Toggle Global Menu", 8*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.ClickAriaLabelAfterWait("Toggle Global Menu", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.ClickAriaLabelAfterWait(c.navAria, 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals(c.sectionAria, "data-active", "true", 8*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if c.assertJSExpr != "" {
		if err := uitest.AssertJS(sc, 5*time.Second, c.assertJSExpr, c.assertFail); err != nil {
			return testv1.StepRunResult{}, err
		}
	}
	overlaps, err := sc.DetectOverlayOverlaps(5 * time.Second)
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	sc.Logf("OVERLAP: section=%s check=start", c.id)
	if len(overlaps) == 0 {
		sc.Logf("OVERLAP: section=%s none", c.id)
	} else {
		var unexpected []string
		for _, ov := range overlaps {
			line := fmt.Sprintf(
				"OVERLAP: section=%s %s:%s/%s(%s) <-> %s:%s/%s(%s) area=%.1fpx a=%.2f%% b=%.2f%% allowedByMenu=%t",
				c.id,
				blank(ov.AKind),
				blank(ov.AOverlay), blank(ov.ARole), blank(ov.ASection),
				blank(ov.BKind),
				blank(ov.BOverlay), blank(ov.BRole), blank(ov.BSection),
				ov.Intersection, ov.PercentOfA, ov.PercentOfB, ov.AllowedByMenu,
			)
			sc.Logf(line)
			if !ov.AllowedByMenu {
				unexpected = append(unexpected, line)
			}
		}
		if len(unexpected) > 0 {
			return testv1.StepRunResult{}, fmt.Errorf("unexpected overlay overlap(s): %s", strings.Join(unexpected, " | "))
		}
	}
	if err := uitest.CaptureScreenshot(sc, c.screenshot); err != nil {
		return testv1.StepRunResult{}, err
	}
	return testv1.StepRunResult{Report: fmt.Sprintf("section %s navigation verified", c.id)}, nil
}

func blank(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}
