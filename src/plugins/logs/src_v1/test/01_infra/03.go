package infra

import (
	"fmt"
	"os"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Run03Finalize(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	finalTopic, err := sc.NewTopicLogger("logs.test.finalize")
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForMessageAfterAction("logs.test.finalize", "finalize-check", 2*time.Second, func() error {
		return finalTopic.Infof("finalize-check")
	}); err != nil {
		return testv1.StepRunResult{}, err
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	reportPath := testReportPath(repoRoot)
	if _, err := os.Stat(reportPath); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("expected report at %s, but missing", reportPath)
	}

	return testv1.StepRunResult{Report: "Suite finalized. Verification transitioned to NATS topics."}, nil
}
