package taskattach

import (
	"fmt"
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "interactive-task-attach-detach",
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
			if err := rt.StartJoin(""); err != nil {
				return testv1.StepRunResult{}, err
			}

			fixture := support.ResolveSSHFixture()
			addHostCmd := fmt.Sprintf("/repl src_v3 add-host --name %s --host %s --user %s", fixture.Alias, fixture.Host, fixture.User)
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: addHostCmd,
				ExpectRoom: support.CombinePatterns(
					[]string{fmt.Sprintf(`"message":"%s"`, addHostCmd)},
					support.StandardTaskRoomPatterns("exited with code 0."),
				),
				ExpectOutput: support.CombinePatterns(
					[]string{support.PromptLine(addHostCmd)},
					support.StandardTaskOutputPatterns("exited with code 0."),
				),
				Timeout: 40 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, err
			}

			roomSeq, _ := rt.CurrentSeqs()
			probeCommand := fmt.Sprintf("ssh src_v1 probe --host %s --timeout 5s", fixture.Alias)
			if err := rt.SendJoinLine("/" + probeCommand); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.WaitForPatternsAfter(20*time.Second, support.CombinePatterns(
				[]string{fmt.Sprintf(`"message":"/%s"`, probeCommand)},
				support.StandardTaskRoomPatterns(""),
			), roomSeq); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("probe task did not start cleanly: %w", err)
			}
			taskID, err := rt.LatestTaskIDForCommand(probeCommand)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if _, err := rt.WaitForTaskPIDForCommandAfter(probeCommand, 20*time.Second, roomSeq); err != nil {
				return testv1.StepRunResult{}, err
			}

			attachCmd := "/task-attach --task-id " + taskID
			if err := rt.SendJoinLine(attachCmd); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.WaitForOutput(15*time.Second, []string{
				"DIALTONE> Attached to task " + taskID + ".",
				"DIALTONE:" + taskID + ">",
				fmt.Sprintf("Probe target=%s transport=ssh", fixture.Alias),
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("attach output missing attached task stream: %w", err)
			}

			if err := rt.SendJoinLine("/task-detach"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.WaitForOutput(10*time.Second, []string{
				"DIALTONE> Detached from task " + taskID + ".",
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("detach output missing: %w", err)
			}
			if err := rt.WaitForOutput(20*time.Second, []string{
				"DIALTONE> Task " + taskID + " exited with code 0.",
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("probe exit lifecycle missing after detach: %w", err)
			}

			ctx.TestPassf("attached to task %s and detached cleanly during real ssh probe for %s", taskID, fixture.Alias)
			return testv1.StepRunResult{
				Report: "Joined REPL with the default hostname prompt, started a real ssh probe as a task, attached the console to the task room with /task-attach --task-id, observed live task output, then detached and confirmed the shared room still reported the final task exit.",
			}, nil
		},
	})
}
