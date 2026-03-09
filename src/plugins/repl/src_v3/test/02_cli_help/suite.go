package clihelp

import (
	"fmt"
	"strings"
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "dialtone-help-surfaces",
		Timeout: 60 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := support.NewRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}

			out, err := rt.RunDialtone("--stdout", "help")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("./dialtone.sh help failed: %w\n%s", err, out)
			}
			mustContain := []string{
				"Usage: ./dialtone.sh <command> [options]",
				"Dev orchestrator commands:",
			}
			for _, p := range mustContain {
				if !strings.Contains(out, p) {
					return testv1.StepRunResult{}, fmt.Errorf("./dialtone.sh help output missing %q", p)
				}
			}

			out, err = rt.RunDialtone("--stdout", "repl", "src_v3", "help")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("./dialtone.sh repl src_v3 help failed: %w\n%s", err, out)
			}
			mustContain = []string{
				"Commands (src_v3):",
				"process-clean",
				"bootstrap",
				"inject --user NAME",
			}
			for _, p := range mustContain {
				if !strings.Contains(out, p) {
					return testv1.StepRunResult{}, fmt.Errorf("repl src_v3 help output missing %q", p)
				}
			}

			ctx.TestPassf("verified dialtone and repl src_v3 help output")
			return testv1.StepRunResult{
				Report: "Verified help surfaces through ./dialtone.sh help and ./dialtone.sh repl src_v3 help in tmp bootstrap workspace.",
			}, nil
		},
	})
}
