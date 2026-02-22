package natswaitpatterns

import (
	"fmt"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:           "nats-step-wait-patterns",
		RunWithContext: runWaitPatterns,
	})
}

func runWaitPatterns(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	if sc.NATSConn() == nil {
		return testv1.StepRunResult{}, fmt.Errorf("expected NATS connection in step context")
	}
	if sc.NATSURL() == "" {
		return testv1.StepRunResult{}, fmt.Errorf("expected NATS URL in step context")
	}
	sc.ResetStepLogClock()

	if err := sc.WaitForStepMessageAfterAction("step-msg-one", 4*time.Second, func() error {
		sc.Infof("step-msg-one")
		return nil
	}); err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.WaitForErrorMessageAfterAction("expected-step-error", 4*time.Second, func() error {
		sc.Errorf("expected-step-error")
		return nil
	}); err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.WaitForAllMessagesAfterAction(sc.StepSubject, []string{"multi-a", "multi-b"}, 5*time.Second, func() error {
		sc.Infof("multi-a")
		sc.Infof("multi-b")
		return nil
	}); err != nil {
		return testv1.StepRunResult{}, err
	}

	custom, err := sc.NewTopicLogger(sc.StepSubject + ".custom")
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForMessageAfterAction(sc.StepSubject+".custom", "custom-topic-hit", 4*time.Second, func() error {
		return custom.Infof("custom-topic-hit")
	}); err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.WaitForMessageAfterAction(sc.StepSubject, "direct-step-hit", 4*time.Second, func() error {
		sc.Infof("direct-step-hit")
		return nil
	}); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("direct wait-for-message pattern failed: %w", err)
	}

	return testv1.StepRunResult{Report: "StepContext NATS wait patterns verified (step/error/custom/all)"}, nil
}
