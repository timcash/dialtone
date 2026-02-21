package infra

import (
	"fmt"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Run01EmbeddedNATSAndPublish(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	infoTopic, err := sc.NewTopicLogger("logs.info.topic")
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	errorTopic, err := sc.NewTopicLogger("logs.error.topic")
	if err != nil {
		return testv1.StepRunResult{}, err
	}

	if err := sc.WaitForMessageAfterAction("logs.info.topic", "startup ok", 2*time.Second, func() error {
		return infoTopic.Infof("startup ok")
	}); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("info message verification failed: %v", err)
	}
	if err := sc.WaitForMessageAfterAction("logs.info.topic", "|INFO|", 2*time.Second, func() error {
		return infoTopic.Infof("info level check")
	}); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("info level prefix missing: %v", err)
	}
	if err := sc.WaitForMessageAfterAction("logs.error.topic", "boom happened", 2*time.Second, func() error {
		return errorTopic.Errorf("boom happened")
	}); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("error message verification failed: %v", err)
	}
	if err := sc.WaitForMessageAfterAction("logs.error.topic", "|ERROR|", 2*time.Second, func() error {
		return errorTopic.Errorf("error level check")
	}); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("error level prefix missing: %v", err)
	}

	return testv1.StepRunResult{Report: fmt.Sprintf("NATS messages verified at %s.", sc.NATSURL())}, nil
}
