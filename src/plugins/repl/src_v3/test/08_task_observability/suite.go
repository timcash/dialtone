package taskobservability

import (
	"fmt"
	"strconv"
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "task-list-and-log-match-real-command",
		Timeout: 180 * time.Second,
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
			hostName := "obs"
			hostAddr := fixture.Host
			hostUser := fixture.User

			cmdText := fmt.Sprintf("repl src_v3 add-host --name %s --host %s --user %s", hostName, hostAddr, hostUser)
			addHostCmd := "/" + cmdText
			roomSeq, _ := rt.CurrentSeqs()
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
				Timeout: 45 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("repl add-host task setup failed: %w", err)
			}
			taskID, err := rt.WaitForTaskIDForCommandAfter(cmdText, 20*time.Second, roomSeq)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing add-host task id: %w", err)
			}
			pid, err := rt.WaitForTaskPIDForCommandAfter(cmdText, 20*time.Second, roomSeq)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing add-host task pid: %w", err)
			}

			listCmd := "/repl src_v3 task list --count 20"
			roomSeq, _ = rt.CurrentSeqs()
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: listCmd,
				ExpectRoom: support.CombinePatterns(
					[]string{fmt.Sprintf(`"message":"%s"`, listCmd)},
					support.StandardTaskRoomPatterns("exited with code 0."),
				),
				ExpectOutput: support.CombinePatterns(
					[]string{support.PromptLine(listCmd)},
					support.StandardTaskOutputPatterns("exited with code 0."),
				),
				Timeout: 30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("repl task list failed: %w", err)
			}
			listTaskID, err := rt.WaitForTaskIDForCommandAfter("repl src_v3 task list --count 20", 20*time.Second, roomSeq)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing task list task id: %w", err)
			}
			if _, err := rt.WaitForTaskPIDForCommandAfter("repl src_v3 task list --count 20", 20*time.Second, roomSeq); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing task list pid: %w", err)
			}
			if err := rt.WaitForSubjectPatterns(fmt.Sprintf("repl.topic.task.%s", listTaskID), 20*time.Second, []string{
				`"scope":"task"`,
				`TASK ID`,
				`STATE`,
				taskID,
				strconv.Itoa(pid),
				`add-host`,
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("task list output did not expose add-host task %s pid %d: %w", taskID, pid, err)
			}

			logCmd := fmt.Sprintf("/repl src_v3 task log --task-id %s --lines 200", taskID)
			roomSeq, _ = rt.CurrentSeqs()
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: logCmd,
				ExpectRoom: support.CombinePatterns(
					[]string{fmt.Sprintf(`"message":"%s"`, logCmd)},
					support.StandardTaskRoomPatterns("exited with code 0."),
				),
				ExpectOutput: support.CombinePatterns(
					[]string{support.PromptLine(logCmd)},
					support.StandardTaskOutputPatterns("exited with code 0."),
				),
				Timeout: 30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("repl task log failed for task %s: %w", taskID, err)
			}
			logTaskID, err := rt.WaitForTaskIDForCommandAfter(fmt.Sprintf("repl src_v3 task log --task-id %s --lines 200", taskID), 20*time.Second, roomSeq)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing task log task id: %w", err)
			}
			if _, err := rt.WaitForTaskPIDForCommandAfter(fmt.Sprintf("repl src_v3 task log --task-id %s --lines 200", taskID), 20*time.Second, roomSeq); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing task log pid: %w", err)
			}
			if err := rt.WaitForSubjectPatterns(fmt.Sprintf("repl.topic.task.%s", logTaskID), 20*time.Second, []string{
				`"scope":"task"`,
				`Task log:`,
				taskID + `.log`,
				`args=`,
				`add-host`,
				hostName,
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("task log output for task %s missing required bits: %w", taskID, err)
			}

			ctx.TestPassf("task list and task log resolved task %s pid %d for %s", taskID, pid, cmdText)
			return testv1.StepRunResult{
				Report: "Ran a real add-host task with the default hostname prompt, verified the task-first lifecycle in the shared REPL transcript, then used task list and task log --task-id to map the task id back to the exact command log.",
			}, nil
		},
	})
}
