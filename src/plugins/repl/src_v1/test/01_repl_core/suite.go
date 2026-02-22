package replcore

import (
	"fmt"
	"strings"
	"time"

	support "dialtone/dev/plugins/repl/src_v1/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name: "repl-help-and-format",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			out, relayed, err := support.RunSessionWithInput(ctx, "help\nexit\n")
			if err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := support.RequireContainsAll(out, []string{
				"DIALTONE> Virtual Librarian online.",
				"DIALTONE> Type 'help' for commands, or 'exit' to quit.",
				"USER-1> help",
				"DIALTONE> Help",
				"`dev install`",
				"`robot src_v1 install`",
				"`dag src_v3 install`",
				"`logs src_v1 test`",
				"`ps`",
				"`kill <pid>`",
				"DIALTONE> Goodbye.",
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			for _, line := range strings.Split(out, "\n") {
				if strings.Contains(line, "DIALTONE>") && !strings.HasPrefix(line, "DIALTONE> ") {
					return testv1.StepRunResult{}, fmt.Errorf("repl output line has invalid DIALTONE format: %q", line)
				}
			}

			if !support.ContainsAny(relayed, "DIALTONE> Help") {
				return testv1.StepRunResult{}, fmt.Errorf("expected help output in REPL relay log")
			}
			if !support.ContainsAny(relayed, "USER-1> help") {
				return testv1.StepRunResult{}, fmt.Errorf("expected USER-1 input in REPL relay log")
			}

			if err := ctx.WaitForStepMessageAfterAction("repl-core-help-format-ok", 3*time.Second, func() error {
				ctx.Infof("repl-core-help-format-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "repl help, input handling, and line format verified"}, nil
		},
	})
}
