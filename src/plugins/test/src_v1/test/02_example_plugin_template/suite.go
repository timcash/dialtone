package exampleplugintemplate

import (
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name: "example-template-step",
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			if err := sc.WaitForStepMessageAfterAction("template plugin info", 4*time.Second, func() error {
				sc.Infof("template plugin info")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := sc.WaitForStepMessageAfterAction("template plugin error", 4*time.Second, func() error {
				sc.Errorf("template plugin error")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "template-style test step ran in shared process"}, nil
		},
	})
}
