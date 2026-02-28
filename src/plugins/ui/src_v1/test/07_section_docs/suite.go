package sectiondocsviamenu

import (
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	sectionsnav "dialtone/dev/plugins/ui/src_v1/test/sections_navigation_lib"
)

func Register(reg *testv1.Registry) {
	tc := sectionsnav.SectionCase{
		ID:          "docs",
		NavAria:     "Navigate Docs",
		SectionAria: "Docs Section",
		Screenshot:  "ui_docs.png",
		AssertJSExpr: `(() => {
			const s = document.getElementById('docs');
			return !!s && s.classList.contains('fullscreen') && !!s.querySelector('header.text');
		})()`,
		AssertFail: "docs should be fullscreen with text header",
	}
	reg.Add(testv1.Step{
		Name:        "ui-section-docs-via-menu",
		Timeout:     5 * time.Second,
		Screenshots: []string{"plugins/ui/src_v1/test/screenshots/ui_docs.png"},
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			return sectionsnav.RunSectionFromMenu(sc, tc, false)
		},
	})
}
