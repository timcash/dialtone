package sectionthreecalculatorviamenu

import (
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:    "ui-section-three-calculator-via-menu",
		Timeout: 10 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			return testv1.StepRunResult{Report: "skipped: fixture no longer exposes a dedicated three-calculator section"}, nil
		},
	})
}
