package subtoneattach

import (
	"fmt"
	"strconv"
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "interactive-subtone-attach-detach",
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
					Send:         "/repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user",
					ExpectRoom:   support.StandardSubtoneRoomPatterns("repl src_v3", ""),
					ExpectOutput: support.StandardSubtoneOutputPatterns("repl src_v3", "DIALTONE> Subtone for repl src_v3 exited with code 0."),
					Timeout:      40 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := rt.SendJoinLine("/ssh src_v1 probe --host wsl --timeout 5s"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.WaitForPatterns(20*time.Second, []string{
				`"scope":"index"`,
				`Request received. Spawning subtone for ssh src_v1`,
				`Subtone started as pid `,
				`Subtone room: subtone-`,
				`Subtone log file: `,
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("probe subtone did not start cleanly: %w", err)
			}
			pid, err := rt.LatestSubtonePID()
			if err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := rt.SendJoinLine("/subtone-attach --pid " + strconv.Itoa(pid)); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.WaitForOutput(15*time.Second, []string{
				fmt.Sprintf("DIALTONE> Attached to subtone-%d.", pid),
				fmt.Sprintf("DIALTONE:%d>", pid),
				"Probe target=wsl transport=ssh",
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("attach output missing attached subtone stream: %w", err)
			}

			if err := rt.SendJoinLine("/subtone-detach"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.WaitForOutput(10*time.Second, []string{
				fmt.Sprintf("DIALTONE> Detached from subtone-%d.", pid),
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("detach output missing: %w", err)
			}
			if err := rt.WaitForOutput(20*time.Second, []string{
				"DIALTONE> Subtone for ssh src_v1 exited with code 0.",
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("probe exit lifecycle missing after detach: %w", err)
			}

			ctx.TestPassf("attached to subtone pid %d and detached cleanly during real ssh probe", pid)
			return testv1.StepRunResult{
				Report: "Joined REPL as llm-codex, started a real ssh probe subtone, attached the console to repl.subtone.<pid>, observed live attached output, then detached and confirmed the index room still reported the final lifecycle exit.",
			}, nil
		},
	})
}
