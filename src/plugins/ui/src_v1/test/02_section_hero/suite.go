package sectionheroviamenu

import (
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	sectionsnav "dialtone/dev/plugins/ui/src_v1/test/sections_navigation_lib"
)

func Register(reg *testv1.Registry) {
	tc := sectionsnav.SectionCase{
		ID:          "ui-hero-stage",
		NavAria:     "Navigate Hero",
		SectionAria: "Hero Section",
		Screenshot:  "ui_hero_section.png",
		AssertJSExpr: `(() => {
			const s = document.getElementById('ui-hero-stage');
			return !!s && s.classList.contains('fullscreen') && !!s.querySelector('header.legend');
		})()`,
		AssertFail: "hero should be fullscreen with legend header",
	}
	reg.Add(testv1.Step{
		Name:    "ui-section-hero-via-menu",
		Timeout: 10 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			return sectionsnav.RunSectionFromMenu(sc, tc, true)
		},
	})
}
