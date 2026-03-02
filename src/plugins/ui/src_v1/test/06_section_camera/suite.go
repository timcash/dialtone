package sectioncameraviamenu

import (
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	sectionsnav "dialtone/dev/plugins/ui/src_v1/test/sections_navigation_lib"
)

func Register(reg *testv1.Registry) {
	tc := sectionsnav.SectionCase{
		ID:          "ui-camera-video",
		NavAria:     "Navigate Camera",
		SectionAria: "Camera Section",
		Screenshot:  "ui_camera.png",
	}
	reg.Add(testv1.Step{
		Name:    "ui-section-camera-via-menu",
		Timeout: 10 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			return sectionsnav.RunSectionFromMenu(sc, tc, false)
		},
	})
}
