package infra

import (
	"fmt"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Run02ErrorTopicFiltering(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	errorTopic, err := sc.NewTopicLogger("logs.error.topic")
	if err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.WaitForMessageAfterAction("logs.error.topic", "filtered error captured", 2*time.Second, func() error {
		return errorTopic.Errorf("filtered error captured")
	}); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("filtered error verification failed: %v", err)
	}
	if err := sc.WaitForMessageAfterAction("logs.error.topic", "|ERROR|", 2*time.Second, func() error {
		return errorTopic.Errorf("error-level-filter-check")
	}); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("error level prefix missing: %v", err)
	}

	return testv1.StepRunResult{Report: "Verified error-topic filtering via NATS"}, nil
}
