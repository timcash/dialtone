package procplugin

import (
	"fmt"
	"time"

	support "dialtone/dev/plugins/repl/src_v1/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "repl-runs-proc-test-subtone",
		Timeout: 90 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			out, _, err := support.RunSessionWithInput(ctx, "proc src_v1 test\nexit\n")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := support.RequireContainsAll(out, []string{
				"USER-1> proc src_v1 test",
				"DIALTONE> Request received. Spawning subtone for proc src_v1...",
				"DIALTONE> Subtone for proc src_v1 exited with code 0.",
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("proc foundation check failed: %w", err)
			}
			return testv1.StepRunResult{Report: "proc plugin test executed through repl subtone"}, nil
		},
	})
}
