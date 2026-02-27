package sectionsnavigation

import (
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	uitest "dialtone/dev/plugins/ui/src_v1/test"
)

var ctx = uitest.SharedContext()

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:    "ui-section-navigation-via-menu",
		Timeout: 75 * time.Second,
		Screenshots: []string{
			"plugins/ui/src_v1/test/screenshots/ui_docs.png",
			"plugins/ui/src_v1/test/screenshots/ui_table.png",
			"plugins/ui/src_v1/test/screenshots/ui_three_fullscreen.png",
			"plugins/ui/src_v1/test/screenshots/ui_camera.png",
			"plugins/ui/src_v1/test/screenshots/ui_settings.png",
		},
		RunWithContext: runNavigation,
	})
}

func runNavigation(sc *testv1.StepContext) (testv1.StepRunResult, error) {
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
	if err := sc.ClickAriaLabelAfterWait("Navigate Docs", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Docs Section", "data-active", "true", 8*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := uitest.AssertJS(sc, 5*time.Second, `(() => {
		const s = document.getElementById('docs');
		return !!s && s.classList.contains('fullscreen') && !!s.querySelector('header.text');
	})()`, "docs should be fullscreen with text header"); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := uitest.CaptureScreenshot(sc, "ui_docs.png"); err != nil {
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
	if err := uitest.AssertJS(sc, 5*time.Second, `(() => {
		const s = document.getElementById('table');
		return !!s && s.classList.contains('calculator') && !!s.querySelector('header.legend');
	})()`, "table should be calculator with legend header"); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := uitest.CaptureScreenshot(sc, "ui_table.png"); err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.ClickAriaLabelAfterWait("Toggle Global Menu", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.ClickAriaLabelAfterWait("Navigate Three Fullscreen", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Three Fullscreen Section", "data-active", "true", 8*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := uitest.CaptureScreenshot(sc, "ui_three_fullscreen.png"); err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.ClickAriaLabelAfterWait("Toggle Global Menu", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.ClickAriaLabelAfterWait("Navigate Camera", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Camera Section", "data-active", "true", 8*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := uitest.CaptureScreenshot(sc, "ui_camera.png"); err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.ClickAriaLabelAfterWait("Toggle Global Menu", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.ClickAriaLabelAfterWait("Navigate Settings", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Settings Section", "data-active", "true", 8*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := uitest.AssertJS(sc, 5*time.Second, `(() => {
		const s = document.getElementById('settings');
		return !!s && s.classList.contains('fullscreen') && !!s.querySelector('header.text');
	})()`, "settings should be fullscreen with text header"); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := uitest.CaptureScreenshot(sc, "ui_settings.png"); err != nil {
		return testv1.StepRunResult{}, err
	}

	return testv1.StepRunResult{Report: "menu navigation verified for docs/table/three-fullscreen/camera/settings with screenshots"}, nil
}
