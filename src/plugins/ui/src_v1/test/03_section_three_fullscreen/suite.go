package sectionthreefullscreenviamenu

import (
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	sectionsnav "dialtone/dev/plugins/ui/src_v1/test/sections_navigation_lib"
)

func Register(reg *testv1.Registry) {
	tc := sectionsnav.SectionCase{
		ID:          "ui-three-fullscreen-stage",
		NavAria:     "Navigate Three Fullscreen",
		SectionAria: "Three Fullscreen Section",
		Screenshot:  "ui_three_fullscreen.png",
	}
	reg.Add(testv1.Step{
		Name:    "ui-section-three-fullscreen-via-menu",
		Timeout: 10 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			return sectionsnav.RunSectionFromMenu(sc, tc, false)
		},
	})
}
