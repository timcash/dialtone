package gobunplugins

import (
	"fmt"
	"strings"
	"time"

	support "dialtone/dev/plugins/repl/src_v1/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "repl-runs-go-and-bun-tests-bottom-up",
		Timeout: 180 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			input := strings.Join([]string{
				"go src_v1 test",
				"bun src_v1 test",
				"exit",
				"",
			}, "\n")
			out, _, err := support.RunSessionWithInput(ctx, input)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := support.RequireContainsAll(out, []string{
				"USER-1> go src_v1 test",
				"DIALTONE> Request received. Spawning subtone for go src_v1...",
				"DIALTONE> Subtone for go src_v1 exited with code 0.",
				"USER-1> bun src_v1 test",
				"DIALTONE> Request received. Spawning subtone for bun src_v1...",
				"DIALTONE> Subtone for bun src_v1 exited with code 0.",
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("go/bun foundation check failed: %w", err)
			}
			return testv1.StepRunResult{Report: "go and bun plugin tests executed through repl subtone flow"}, nil
		},
	})
}
