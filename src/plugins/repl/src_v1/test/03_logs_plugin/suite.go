package logsplugin

import (
	"fmt"
	"time"

	support "dialtone/dev/plugins/repl/src_v1/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "repl-runs-logs-test-subtone",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			out, _, err := support.RunSessionWithInput(ctx, "logs src_v1 test\nexit\n")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := support.RequireContainsAll(out, []string{
				"USER-1> logs src_v1 test",
				"DIALTONE> Request received. Spawning subtone for logs src_v1...",
				"DIALTONE> Subtone for logs src_v1 exited with code 0.",
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("logs foundation check failed: %w", err)
			}
			return testv1.StepRunResult{Report: "logs plugin test executed through repl subtone"}, nil
		},
	})
}
