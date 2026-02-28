package sectionterminalviamenu

import (
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	sectionsnav "dialtone/dev/plugins/ui/src_v1/test/sections_navigation_lib"
)

func Register(reg *testv1.Registry) {
	tc := sectionsnav.SectionCase{
		ID:          "terminal",
		NavAria:     "Navigate Terminal",
		SectionAria: "Terminal Section",
		Screenshot:  "ui_terminal_section.png",
	}
	reg.Add(testv1.Step{
		Name:        "ui-section-terminal-via-menu",
		Timeout:     5 * time.Second,
		Screenshots: []string{"plugins/ui/src_v1/test/screenshots/ui_terminal_section.png"},
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			return sectionsnav.RunSectionFromMenu(sc, tc, false)
		},
	})
}
