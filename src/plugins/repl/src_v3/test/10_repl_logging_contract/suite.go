package replloggingcontract

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "interactive-command-index-lifecycle-contract",
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

			cmd := "/repl src_v3 help"
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: cmd,
				ExpectRoom: support.CombinePatterns(
					[]string{fmt.Sprintf(`"message":"%s"`, cmd)},
					support.StandardTaskRoomPatterns("exited with code 0."),
				),
				ExpectOutput: support.CombinePatterns(
					[]string{support.PromptLine(cmd)},
					support.StandardTaskOutputPatterns("exited with code 0."),
				),
				Timeout: 30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("interactive command lifecycle contract failed: %w", err)
			}

			indexLines := strings.Join(rt.SubjectMessages("repl.topic.index"), "\n")
			for _, forbidden := range []string{
				`Spawning subtone`,
				`Subtone started as pid`,
				`Subtone topic:`,
				`Subtone log file:`,
				`Subtone for repl src_v3 exited`,
			} {
				if strings.Contains(indexLines, forbidden) {
					return testv1.StepRunResult{}, fmt.Errorf("index topic still contains legacy lifecycle text %q\n%s", forbidden, indexLines)
				}
			}

			ctx.TestPassf("interactive routed command emitted a strict task-first index lifecycle")
			return testv1.StepRunResult{
				Report: "Joined REPL with the default hostname prompt, ran `/repl src_v3 help`, and verified the top-level `dialtone>` lifecycle is strictly task-first: request received, task queued, task topic, task log, pid assignment, and exit with no legacy subtone wording.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "interactive-command-index-emits-task-queue-lines",
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

			cmd := "/repl src_v3 help"
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: cmd,
				ExpectRoom: support.CombinePatterns(
					[]string{fmt.Sprintf(`"message":"%s"`, cmd)},
					support.StandardTaskRoomPatterns("exited with code 0."),
				),
				ExpectOutput: support.CombinePatterns(
					[]string{support.PromptLine(cmd)},
					support.StandardTaskOutputPatterns("exited with code 0."),
				),
				Timeout: 30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("interactive command task-first lifecycle failed: %w", err)
			}
			indexLines := strings.Join(rt.SubjectMessages("repl.topic.index"), "\n")
			for _, forbidden := range []string{
				`Spawning subtone`,
				`Subtone started as pid`,
				`Subtone topic:`,
				`Subtone log file:`,
			} {
				if strings.Contains(indexLines, forbidden) {
					return testv1.StepRunResult{}, fmt.Errorf("index topic still contains legacy lifecycle text %q\n%s", forbidden, indexLines)
				}
			}

			ctx.TestPassf("interactive routed command emitted task queue, pid, log, and exit lines")
			return testv1.StepRunResult{
				Report: "Joined REPL with the default hostname prompt, ran `/repl src_v3 help`, and verified the index-topic `dialtone>` transcript includes task queue, task topic, task log, pid assignment, and task exit lines without legacy subtone lifecycle text.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "interactive-foreground-task-topic-payload",
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
			if err := rt.WaitForOutput(10*time.Second, []string{
				`dialtone> Starting test: interactive-foreground-task-topic-payload`,
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("test start lifecycle line missing from REPL output: %w", err)
			}

			cmd := "/repl src_v3 help"
			roomSeq, _ := rt.CurrentSeqs()
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: cmd,
				ExpectRoom: support.CombinePatterns(
					[]string{fmt.Sprintf(`"message":"%s"`, cmd)},
					support.StandardTaskRoomPatterns("exited with code 0."),
				),
				ExpectOutput: support.CombinePatterns(
					[]string{support.PromptLine(cmd)},
					support.StandardTaskOutputPatterns("exited with code 0."),
				),
				Timeout: 30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, err
			}
			taskID, err := rt.WaitForTaskIDForCommandAfter("repl src_v3 help", 20*time.Second, roomSeq)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitForSubjectContains(rt, fmt.Sprintf("repl.topic.task.%s", taskID), 15*time.Second, []string{
				`"scope":"task"`,
				`Usage: ./dialtone.sh repl src_v3 `,
				`Commands (src_v3):`,
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("foreground task topic payload missing: %w", err)
			}
			ctx.TestPassf("foreground task lifecycle and task-topic payload validated through REPL output")

			return testv1.StepRunResult{
				Report: "Ran `/repl src_v3 help` as a foreground task and verified the full index-topic task lifecycle plus the per-task topic help payload.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "index-topic-does-not-mirror-task-topic-payload",
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

			cmd := "/repl src_v3 help"
			roomSeq, _ := rt.CurrentSeqs()
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: cmd,
				ExpectRoom: support.CombinePatterns(
					[]string{fmt.Sprintf(`"message":"%s"`, cmd)},
					support.StandardTaskRoomPatterns("exited with code 0."),
				),
				ExpectOutput: support.CombinePatterns(
					[]string{support.PromptLine(cmd)},
					support.StandardTaskOutputPatterns("exited with code 0."),
				),
				Timeout: 30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, err
			}
			taskID, err := rt.WaitForTaskIDForCommandAfter("repl src_v3 help", 20*time.Second, roomSeq)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			subject := fmt.Sprintf("repl.topic.task.%s", taskID)
			if err := waitForSubjectContains(rt, subject, 15*time.Second, []string{
				`"scope":"task"`,
				`Usage: ./dialtone.sh repl src_v3 `,
				`Commands (src_v3):`,
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			indexLines := strings.Join(rt.SubjectMessages("repl.topic.index"), "\n")
			for _, forbidden := range []string{
				`Usage: ./dialtone.sh repl src_v3 `,
				`Commands (src_v3):`,
			} {
				if strings.Contains(indexLines, forbidden) {
					return testv1.StepRunResult{}, fmt.Errorf("index topic mirrored subtone payload %q\n%s", forbidden, indexLines)
				}
			}

			return testv1.StepRunResult{
				Report: "Ran a local foreground task and verified the detailed help payload stayed in `repl.topic.task.<task-id>` rather than leaking into `repl.topic.index`.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "interactive-background-task-lifecycle",
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

			_, pid, err := startBackgroundWatch(rt, "bg")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send:         "/ps",
				ExpectRoom:   []string{`"type":"input"`, `"message":"/ps"`, `"message":"Running Tasks:"`, strconv.Itoa(pid)},
				ExpectOutput: []string{`dialtone> Running Tasks:`, fmt.Sprintf("%d", pid)},
				Timeout:      20 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := cleanupManagedTasks(rt); err != nil {
				return testv1.StepRunResult{}, err
			}

			return testv1.StepRunResult{
				Report: "Started a local background watch task through REPL, confirmed `/ps` reported its runtime pid as active, then cleaned the managed processes before the next step.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "ps-matches-live-task-registry",
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

			taskID, pid, err := startBackgroundWatch(rt, "registry")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send:         "/ps",
				ExpectRoom:   []string{`"message":"Running Tasks:"`, strconv.Itoa(pid)},
				ExpectOutput: []string{`dialtone> Running Tasks:`, fmt.Sprintf("%d", pid)},
				Timeout:      20 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, err
			}

			listCmd := "/repl src_v3 task list --count 20"
			roomSeq, _ := rt.CurrentSeqs()
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: listCmd,
				ExpectRoom: support.CombinePatterns([]string{
					fmt.Sprintf(`"message":"%s"`, listCmd),
				}, support.StandardTaskRoomPatterns("exited with code 0.")),
				ExpectOutput: support.CombinePatterns(
					[]string{support.PromptLine(listCmd)},
					support.StandardTaskOutputPatterns("exited with code 0."),
				),
				Timeout: 30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("repl task list command failed: %w", err)
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
				`running`,
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("task list output did not expose running row for task %s pid %d: %w", taskID, pid, err)
			}

			logCmd := fmt.Sprintf("/repl src_v3 task log --task-id %s --lines 50", taskID)
			roomSeq, _ = rt.CurrentSeqs()
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: logCmd,
				ExpectRoom: support.CombinePatterns([]string{
					fmt.Sprintf(`"message":"%s"`, logCmd),
				}, support.StandardTaskRoomPatterns("exited with code 0.")),
				ExpectOutput: support.CombinePatterns(
					[]string{support.PromptLine(logCmd)},
					support.StandardTaskOutputPatterns("exited with code 0."),
				),
				Timeout: 30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("repl task log command failed for task %s: %w", taskID, err)
			}
			logTaskID, err := rt.WaitForTaskIDForCommandAfter(fmt.Sprintf("repl src_v3 task log --task-id %s --lines 50", taskID), 20*time.Second, roomSeq)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing task log task id: %w", err)
			}
			if _, err := rt.WaitForTaskPIDForCommandAfter(fmt.Sprintf("repl src_v3 task log --task-id %s --lines 50", taskID), 20*time.Second, roomSeq); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing task log pid: %w", err)
			}
			if err := rt.WaitForSubjectPatterns(fmt.Sprintf("repl.topic.task.%s", logTaskID), 20*time.Second, []string{
				`"scope":"task"`,
				"Task log:",
				taskID + ".log",
				"watch",
				"--subject",
				"repl.topic.index",
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("task log output for task %s missing required bits: %w", taskID, err)
			}
			if err := cleanupManagedTasks(rt); err != nil {
				return testv1.StepRunResult{}, err
			}

			return testv1.StepRunResult{
				Report: "Started a local background task, confirmed `/ps`, `task list`, and `task log --task-id` all agreed on the live task registry state, then cleaned running tasks before the next step.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "interactive-nonzero-exit-lifecycle",
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

			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: "/repl src_v3 definitely-not-a-real-command",
				ExpectRoom: support.CombinePatterns(
					[]string{
						`"type":"input"`,
						`"message":"/repl src_v3 definitely-not-a-real-command"`,
					},
					support.StandardTaskRoomPatterns("exited with code 1."),
				),
				ExpectOutput: support.CombinePatterns(
					[]string{
						support.PromptLine("/repl src_v3 definitely-not-a-real-command"),
					},
					support.StandardTaskOutputPatterns("exited with code 1."),
				),
				Timeout: 20 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, err
			}
			taskID, err := rt.LatestTaskID()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitForSubjectContains(rt, fmt.Sprintf("repl.topic.task.%s", taskID), 10*time.Second, []string{
				`"scope":"task"`,
				`Unsupported repl src_v3 command: definitely-not-a-real-command`,
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("nonzero task error payload missing: %w", err)
			}

			return testv1.StepRunResult{
				Report: "Ran an invalid REPL command and verified the index topic reported a task-first nonzero exit while the per-task topic retained the detailed error payload.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "multiple-concurrent-background-tasks",
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

			firstTaskID, firstPID, err := startBackgroundWatch(rt, "alpha")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			secondTaskID, secondPID, err := startBackgroundWatch(rt, "beta")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if firstTaskID == secondTaskID {
				return testv1.StepRunResult{}, fmt.Errorf("expected distinct background task ids, got %q", firstTaskID)
			}
			if firstPID == secondPID {
				return testv1.StepRunResult{}, fmt.Errorf("expected distinct background runtime pids, got %d", firstPID)
			}

			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send:         "/ps",
				ExpectRoom:   []string{`"message":"Running Tasks:"`, strconv.Itoa(firstPID), strconv.Itoa(secondPID)},
				ExpectOutput: []string{`dialtone> Running Tasks:`, strconv.Itoa(firstPID), strconv.Itoa(secondPID)},
				Timeout:      20 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, err
			}
			for _, spec := range []struct {
				taskID string
				pid    int
				filter string
			}{
				{firstTaskID, firstPID, "alpha"},
				{secondTaskID, secondPID, "beta"},
			} {
				if err := waitForSubjectContains(rt, fmt.Sprintf("repl.topic.task.%s", spec.taskID), 10*time.Second, []string{
					`"scope":"task"`,
					`watching NATS subject`,
					fmt.Sprintf(`--filter %s`, spec.filter),
				}); err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("task %s (pid %d) did not retain isolated command payload: %w", spec.taskID, spec.pid, err)
				}
			}
			if err := cleanupManagedTasks(rt); err != nil {
				return testv1.StepRunResult{}, err
			}

			return testv1.StepRunResult{
				Report: "Started two concurrent background watch tasks, verified `/ps` showed both runtime pids, confirmed each task topic kept its own command payload, then cleaned managed processes before the next step.",
			}, nil
		},
	})
}

func startBackgroundWatch(rt *support.Runtime, filter string) (string, int, error) {
	prevPID, _ := rt.LatestSubtonePID()
	commandText := fmt.Sprintf("repl src_v3 watch --nats-url %s --subject repl.topic.index --filter %s", rt.NATSURL, strings.TrimSpace(filter))
	cmd := "/" + commandText + " &"
	roomSeq, _ := rt.CurrentSeqs()
	if err := rt.SendJoinLine(cmd); err != nil {
		return "", 0, err
	}
	if err := rt.WaitForPatterns(20*time.Second, support.CombinePatterns(
		[]string{
			fmt.Sprintf(`--filter %s`, strings.TrimSpace(filter)),
		},
		support.StandardTaskRoomPatterns("is running in background."),
	)); err != nil {
		return "", 0, fmt.Errorf("background watch task did not start cleanly: %w", err)
	}
	taskID, err := rt.WaitForTaskIDForCommandAfter(commandText, 10*time.Second, roomSeq)
	if err != nil {
		return "", 0, fmt.Errorf("background watch task id missing for command %q: %w", commandText, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	pid := 0
	for time.Now().Before(deadline) {
		nextPID, err := rt.LatestSubtonePID()
		if err == nil && nextPID > 0 && nextPID != prevPID {
			pid = nextPID
			break
		}
		time.Sleep(120 * time.Millisecond)
	}
	if pid <= 0 {
		return "", 0, fmt.Errorf("no new background runtime pid observed after %q", cmd)
	}
	if err := waitForSubjectContains(rt, fmt.Sprintf("repl.topic.task.%s", taskID), 10*time.Second, []string{
		`"scope":"task"`,
		`watching NATS subject`,
		fmt.Sprintf(`--filter %s`, strings.TrimSpace(filter)),
	}); err != nil {
		return "", 0, err
	}
	return taskID, pid, nil
}

func cleanupManagedTasks(rt *support.Runtime) error {
	listCmd := "/repl src_v3 task list --count 50 --state running"
	roomSeq, _ := rt.CurrentSeqs()
	if err := rt.RunTranscript([]support.TranscriptStep{{
		Send: listCmd,
		ExpectRoom: support.CombinePatterns([]string{
			fmt.Sprintf(`"message":"%s"`, listCmd),
		}, support.StandardTaskRoomPatterns("exited with code 0.")),
		ExpectOutput: support.CombinePatterns(
			[]string{support.PromptLine(listCmd)},
			support.StandardTaskOutputPatterns("exited with code 0."),
		),
		Timeout: 30 * time.Second,
	}}); err != nil {
		return fmt.Errorf("repl task list cleanup command failed: %w", err)
	}
	taskID, err := rt.WaitForTaskIDForCommandAfter("repl src_v3 task list --count 50 --state running", 20*time.Second, roomSeq)
	if err != nil {
		return fmt.Errorf("missing cleanup task list task id: %w", err)
	}
	listPID, err := rt.WaitForTaskPIDForCommandAfter("repl src_v3 task list --count 50 --state running", 20*time.Second, roomSeq)
	if err != nil {
		return fmt.Errorf("missing cleanup task list pid: %w", err)
	}
	out := strings.Join(rt.SubjectMessages(fmt.Sprintf("repl.topic.task.%s", taskID)), "\n")
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		var frame map[string]any
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			continue
		}
		msgText, ok := frame["message"].(string)
		if !ok {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(msgText))
		if len(fields) < 4 {
			continue
		}
		runningTaskID := strings.TrimSpace(fields[0])
		pid, err := strconv.Atoi(strings.TrimSpace(fields[1]))
		if err != nil || pid <= 0 {
			continue
		}
		if pid == listPID || runningTaskID == taskID || runningTaskID == "-" {
			continue
		}
		if strings.TrimSpace(fields[3]) != "running" {
			continue
		}
		killCmd := fmt.Sprintf("/repl src_v3 task kill --task-id %s", runningTaskID)
		if err := rt.RunTranscript([]support.TranscriptStep{{
			Send: killCmd,
			ExpectRoom: support.CombinePatterns(
				[]string{fmt.Sprintf(`"message":"%s"`, killCmd)},
				support.StandardTaskRoomPatterns("exited with code 0."),
			),
			ExpectOutput: support.CombinePatterns(
				[]string{support.PromptLine(killCmd)},
				support.StandardTaskOutputPatterns("exited with code 0."),
			),
			Timeout: 20 * time.Second,
		}}); err != nil {
			return fmt.Errorf("task kill failed for %s (pid %d): %w", runningTaskID, pid, err)
		}
	}
	return nil
}

func waitForSubjectContains(rt *support.Runtime, subject string, timeout time.Duration, needles []string) error {
	deadline := time.Now().Add(timeout)
	for {
		body := strings.Join(rt.SubjectMessages(subject), "\n")
		missing := make([]string, 0, len(needles))
		for _, needle := range needles {
			if !strings.Contains(body, needle) {
				missing = append(missing, needle)
			}
		}
		if len(missing) == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("subject %s missing %s\n%s", subject, strings.Join(missing, ", "), body)
		}
		time.Sleep(120 * time.Millisecond)
	}
}
