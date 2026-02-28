package sectionsettingsviamenu

import (
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	sectionsnav "dialtone/dev/plugins/ui/src_v1/test/sections_navigation_lib"
)

func Register(reg *testv1.Registry) {
	tc := sectionsnav.SectionCase{
		ID:          "settings",
		NavAria:     "Navigate Settings",
		SectionAria: "Settings Section",
		Screenshot:  "ui_settings.png",
		AssertJSExpr: `(() => {
			const s = document.getElementById('settings');
			return !!s && s.classList.contains('fullscreen') && !!s.querySelector('header.text');
		})()`,
		AssertFail: "settings should be fullscreen with text header",
	}
	reg.Add(testv1.Step{
		Name:        "ui-section-settings-via-menu",
		Timeout:     5 * time.Second,
		Screenshots: []string{"plugins/ui/src_v1/test/screenshots/ui_settings.png"},
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			return sectionsnav.RunSectionFromMenu(sc, tc, false)
		},
	})
}
