package selfcheck

import (
	"fmt"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name: "ctx-logging-and-waits",
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			if err := sc.WaitForStepMessageAfterAction("ctx info message", 4*time.Second, func() error {
				sc.Infof("ctx info message")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := sc.WaitForStepMessageAfterAction("ctx warn message", 4*time.Second, func() error {
				sc.Warnf("ctx warn message")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := sc.WaitForStepMessageAfterAction("ctx error message", 4*time.Second, func() error {
				sc.Errorf("ctx error message")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := sc.WaitForErrorMessageAfterAction("ctx error message", 4*time.Second, func() error {
				sc.Errorf("ctx error message")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := sc.WaitForStepMessageAfterAction("|INFO|", 4*time.Second, func() error {
				sc.Infof("ctx info format check")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := sc.WaitForStepMessageAfterAction("|WARN|", 4*time.Second, func() error {
				sc.Warnf("ctx warn format check")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := sc.WaitForStepMessageAfterAction("|ERROR|", 4*time.Second, func() error {
				sc.Errorf("ctx error format check")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "StepContext log methods + wait helpers verified"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "ctx-subjects-populated",
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			if sc.SuiteSubject == "" || sc.StepSubject == "" || sc.ErrorSubject == "" {
				return testv1.StepRunResult{}, fmt.Errorf("StepContext subjects not populated")
			}
			return testv1.StepRunResult{Report: "StepContext subjects available for plugin tests"}, nil
		},
	})
}
