package chromeplugin

import (
	"fmt"
	"time"

	support "dialtone/dev/plugins/repl/src_v1/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "repl-runs-chrome-foundation-command",
		Timeout: 60 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			out, _, err := support.RunSessionWithInput(ctx, "chrome src_v1 list\nexit\n")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := support.RequireContainsAll(out, []string{
				"USER-1> chrome src_v1 list",
				"DIALTONE> Request received. Spawning subtone for chrome src_v1...",
				"DIALTONE> Subtone for chrome src_v1 exited with code 0.",
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("chrome foundation check failed: %w", err)
			}
			return testv1.StepRunResult{Report: "chrome foundation command executed through repl subtone"}, nil
		},
	})
}
