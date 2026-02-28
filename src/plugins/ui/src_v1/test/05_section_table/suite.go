package sectiontableviamenu

import (
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	sectionsnav "dialtone/dev/plugins/ui/src_v1/test/sections_navigation_lib"
)

func Register(reg *testv1.Registry) {
	tc := sectionsnav.SectionCase{
		ID:          "table",
		NavAria:     "Navigate Table",
		SectionAria: "Table Section",
		Screenshot:  "ui_table.png",
		AssertJSExpr: `(() => {
			const s = document.getElementById('table');
			return !!s && s.classList.contains('fullscreen') && !!s.querySelector('header.legend');
		})()`,
		AssertFail: "table should be fullscreen with legend header",
	}
	reg.Add(testv1.Step{
		Name:    "ui-section-table-via-menu",
		Timeout: 5 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			return sectionsnav.RunSectionFromMenu(sc, tc, false)
		},
	})
}
