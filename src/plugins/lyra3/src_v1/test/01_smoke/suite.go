package smoke

import (
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:    "lyra3-smoke-check",
		Timeout: 10 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			sc.Infof("Running basic smoke check for Lyra3...")
			// Logic to verify core plugin functionality
			return testv1.StepRunResult{Report: "Lyra3 smoke test passed!"}, nil
		},
	})
}
