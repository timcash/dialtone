package replhelpps

import (
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "injected-help-and-ps",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := support.NewRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()
			if err := rt.StartLeader(); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.StartJoin("local-human"); err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := rt.Inject("llm-codex", "help"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.WaitForPatterns(30*time.Second, []string{
				`"message":"System"`,
				`"message":"List active subtones"`,
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := rt.Inject("llm-codex", "ps"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.WaitForPatterns(30*time.Second, []string{
				`"message":"No active subtones."`,
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("help and ps executed through injected command path")
			return testv1.StepRunResult{
				Report: "Injected help and ps commands through REPL command bus and validated room output includes command/help text and ps empty-state response.",
			}, nil
		},
	})
}
