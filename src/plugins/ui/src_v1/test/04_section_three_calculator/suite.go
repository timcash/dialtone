package sectionthreecalculatorviamenu

import (
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	sectionsnav "dialtone/dev/plugins/ui/src_v1/test/sections_navigation_lib"
)

func Register(reg *testv1.Registry) {
	tc := sectionsnav.SectionCase{
		ID:          "three-calculator",
		NavAria:     "Navigate Three Calculator",
		SectionAria: "Three Calculator Section",
		Screenshot:  "ui_three_calculator_section.png",
	}
	reg.Add(testv1.Step{
		Name:        "ui-section-three-calculator-via-menu",
		Timeout:     5 * time.Second,
		Screenshots: []string{"plugins/ui/src_v1/test/screenshots/ui_three_calculator_section.png"},
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			return sectionsnav.RunSectionFromMenu(sc, tc, false)
		},
	})
}
