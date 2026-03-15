package replhelpps

import (
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "interactive-help-and-ps",
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
			if err := rt.StartJoin("llm-codex"); err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send:         "/help",
					ExpectRoom:   []string{`"type":"input"`, `"from":"llm-codex"`, `"message":"/help"`, `"message":"Help"`, `"message":"List active subtones"`},
					ExpectOutput: []string{`DIALTONE> Help`, `DIALTONE> List active subtones`},
					Timeout:      30 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send:         "/ps",
					ExpectRoom:   []string{`"type":"input"`, `"from":"llm-codex"`, `"message":"/ps"`, `"message":"No active subtones."`},
					ExpectOutput: []string{`DIALTONE> No active subtones.`},
					Timeout:      30 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("help and ps executed through llm-codex REPL prompt path")
			return testv1.StepRunResult{
				Report: "Joined REPL as llm-codex, ran /help and /ps through the live prompt path, and validated the room output includes the input events, help text, and ps empty-state response.",
			}, nil
		},
	})
}
