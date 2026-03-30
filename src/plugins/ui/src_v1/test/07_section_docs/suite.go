package sectiondocsviamenu

import (
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	sectionsnav "dialtone/dev/plugins/ui/src_v1/test/sections_navigation_lib"
)

func Register(reg *testv1.Registry) {
	tc := sectionsnav.SectionCase{
		ID:          "ui-home-docs",
		NavAria:     "Open Home",
		SectionAria: "Docs Section",
		Screenshot:  "ui_docs.png",
		AssertJSExpr: `(() => {
			const s = document.getElementById('ui-home-docs');
			return !!s && s.classList.contains('fullscreen') && !!s.querySelector('header.shell-legend-text');
		})()`,
		AssertFail: "docs should be fullscreen with text legend header",
	}
	reg.Add(testv1.Step{
		Name:    "ui-section-docs-via-menu",
		Timeout: sectionsnav.StepTimeout,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			return sectionsnav.RunSectionFromMenu(sc, tc, false)
		},
	})
}
