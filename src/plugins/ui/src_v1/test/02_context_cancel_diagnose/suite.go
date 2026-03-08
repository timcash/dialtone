package contextcanceldiagnose

import (
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	uitest "dialtone/dev/plugins/ui/src_v1/test"
)

var ctx = uitest.SharedContext()

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:    "ui-attach-context-cancel-diagnose",
		Timeout: 5 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			return run(sc)
		},
	})
}

func run(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	ctx.BeginStep(sc)
	opts := uitest.GetOptions()
	attach := strings.TrimSpace(opts.AttachNode) != ""
	if !attach {
		return testv1.StepRunResult{Report: "skipped (not running in --attach mode)"}, nil
	}
	return testv1.StepRunResult{Report: "skipped for chrome src_v3 service-managed attach mode"}, nil
}
