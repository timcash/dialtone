package replloggingcontract

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "interactive-foreground-subtone-lifecycle",
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
			if err := rt.WaitForOutput(10*time.Second, []string{
				`DIALTONE> Starting test: interactive-foreground-subtone-lifecycle`,
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("test start lifecycle line missing from REPL output: %w", err)
			}

			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send:         "/repl src_v3 help",
				ExpectRoom:   support.StandardSubtoneRoomPatterns("repl src_v3", ""),
				ExpectOutput: append([]string{`llm-codex> /repl src_v3 help`}, support.StandardSubtoneOutputPatterns("repl src_v3", "DIALTONE> Subtone for repl src_v3 exited with code 0.")...),
				Timeout:      30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, err
			}
			pid, err := rt.LatestSubtonePID()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitForSubjectContains(rt, fmt.Sprintf("repl.subtone.%d", pid), 15*time.Second, []string{
				`"scope":"subtone"`,
				`Usage: ./dialtone.sh repl src_v3 `,
				`Commands (src_v3):`,
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("foreground subtone payload missing: %w", err)
			}
			ctx.TestPassf("foreground subtone lifecycle validated through REPL output")

			return testv1.StepRunResult{
				Report: "Ran `/repl src_v3 help` as a foreground subtone and verified the full index-room lifecycle plus subtone-room help payload.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "main-room-does-not-mirror-subtone-payload",
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

			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send:         "/repl src_v3 help",
				ExpectRoom:   support.StandardSubtoneRoomPatterns("repl src_v3", ""),
				ExpectOutput: support.StandardSubtoneOutputPatterns("repl src_v3", "DIALTONE> Subtone for repl src_v3 exited with code 0."),
				Timeout:      30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, err
			}
			pid, err := rt.LatestSubtonePID()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			subject := fmt.Sprintf("repl.subtone.%d", pid)
			if err := waitForSubjectContains(rt, subject, 15*time.Second, []string{
				`Usage: ./dialtone.sh repl src_v3 `,
				`Commands (src_v3):`,
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			indexLines := strings.Join(rt.SubjectMessages("repl.room.index"), "\n")
			for _, forbidden := range []string{
				`Usage: ./dialtone.sh repl src_v3 `,
				`Commands (src_v3):`,
			} {
				if strings.Contains(indexLines, forbidden) {
					return testv1.StepRunResult{}, fmt.Errorf("index room mirrored subtone payload %q\n%s", forbidden, indexLines)
				}
			}

			return testv1.StepRunResult{
				Report: "Ran a noisy local subtone and verified detailed help payload stayed in `repl.subtone.<pid>` rather than leaking into `repl.room.index`.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "interactive-background-subtone-lifecycle",
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

			pid, err := startBackgroundWatch(rt, "bg")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send:         "/ps",
				ExpectRoom:   []string{`"type":"input"`, `"message":"/ps"`, `"message":"Active Subtones:"`, fmt.Sprintf(`"message":"%-8d`, pid)},
				ExpectOutput: []string{`DIALTONE> Active Subtones:`, fmt.Sprintf("%d", pid)},
				Timeout:      20 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := cleanupManagedSubtones(rt); err != nil {
				return testv1.StepRunResult{}, err
			}

			return testv1.StepRunResult{
				Report: "Started a local background watch subtone through REPL, confirmed `/ps` reported it as active, then cleaned the managed processes before the next step.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "ps-matches-live-subtone-registry",
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

			pid, err := startBackgroundWatch(rt, "registry")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send:         "/ps",
				ExpectRoom:   []string{`"message":"Active Subtones:"`, fmt.Sprintf(`"message":"%-8d`, pid)},
				ExpectOutput: []string{`DIALTONE> Active Subtones:`, fmt.Sprintf("%d", pid)},
				Timeout:      20 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, err
			}

			listCmd := "/repl src_v3 subtone-list --count 20"
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: listCmd,
				ExpectRoom: support.CombinePatterns([]string{
					fmt.Sprintf(`"message":"%s"`, listCmd),
				}, support.StandardSubtoneRoomPatterns("repl src_v3", "")),
				ExpectOutput: append([]string{fmt.Sprintf("llm-codex> %s", listCmd)}, support.StandardSubtoneOutputPatterns("repl src_v3", "")...),
				Timeout:      30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("repl subtone-list failed: %w", err)
			}
			listPID, err := rt.WaitForSubtonePIDForCommand("repl src_v3 subtone-list --count 20", 20*time.Second)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing subtone-list pid: %w", err)
			}
			if err := rt.WaitForSubjectPatterns(fmt.Sprintf("repl.subtone.%d", listPID), 20*time.Second, []string{
				`STATE`,
				strconv.Itoa(pid),
				`active`,
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("subtone-list did not expose active registry row for pid %d: %w", pid, err)
			}

			logCmd := fmt.Sprintf("/repl src_v3 subtone-log --pid %d --lines 50", pid)
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: logCmd,
				ExpectRoom: support.CombinePatterns([]string{
					fmt.Sprintf(`"message":"%s"`, logCmd),
				}, support.StandardSubtoneRoomPatterns("repl src_v3", "")),
				ExpectOutput: append([]string{fmt.Sprintf("llm-codex> %s", logCmd)}, support.StandardSubtoneOutputPatterns("repl src_v3", "")...),
				Timeout:      30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("repl subtone-log failed for pid %d: %w", pid, err)
			}
			logPID, err := rt.WaitForSubtonePIDForCommand(fmt.Sprintf("repl src_v3 subtone-log --pid %d --lines 50", pid), 20*time.Second)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing subtone-log pid: %w", err)
			}
			if err := rt.WaitForSubjectPatterns(fmt.Sprintf("repl.subtone.%d", logPID), 20*time.Second, []string{
				"Subtone log:",
				"watch",
				"--subject",
				"repl.room.index",
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("subtone-log output for pid %d missing required bits: %w", pid, err)
			}
			if err := cleanupManagedSubtones(rt); err != nil {
				return testv1.StepRunResult{}, err
			}

			return testv1.StepRunResult{
				Report: "Started a local background subtone, confirmed `/ps`, `subtone-list`, and `subtone-log --pid` all agreed on the live registry state, then cleaned managed processes before the next step.",
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
			if err := rt.StartJoin("llm-codex"); err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: "/repl src_v3 definitely-not-a-real-command",
				ExpectRoom: []string{
					`"type":"input"`,
					`"message":"/repl src_v3 definitely-not-a-real-command"`,
					`Request received. Spawning subtone for repl src_v3`,
					`Subtone started as pid `,
					`Subtone room: subtone-`,
					`Subtone for repl src_v3 exited with code 1.`,
				},
				ExpectOutput: []string{
					`DIALTONE> Request received. Spawning subtone for repl src_v3`,
					`DIALTONE> Subtone started as pid `,
					`DIALTONE> Subtone for repl src_v3 exited with code 1.`,
				},
				Timeout: 20 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, err
			}
			pid, err := rt.LatestSubtonePID()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitForSubjectContains(rt, fmt.Sprintf("repl.subtone.%d", pid), 10*time.Second, []string{
				`Unsupported repl src_v3 command: definitely-not-a-real-command`,
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("nonzero subtone error payload missing: %w", err)
			}

			return testv1.StepRunResult{
				Report: "Ran an invalid REPL subtone command and verified the index room reported a nonzero exit while the subtone room retained the detailed error payload.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "multiple-concurrent-background-subtones",
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

			firstPID, err := startBackgroundWatch(rt, "alpha")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			secondPID, err := startBackgroundWatch(rt, "beta")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if firstPID == secondPID {
				return testv1.StepRunResult{}, fmt.Errorf("expected distinct background subtone pids, got %d", firstPID)
			}

			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send:         "/ps",
				ExpectRoom:   []string{`"message":"Active Subtones:"`, fmt.Sprintf(`"message":"%-8d`, firstPID), fmt.Sprintf(`"message":"%-8d`, secondPID)},
				ExpectOutput: []string{`DIALTONE> Active Subtones:`, strconv.Itoa(firstPID), strconv.Itoa(secondPID)},
				Timeout:      20 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, err
			}
			for _, spec := range []struct {
				pid    int
				filter string
			}{
				{firstPID, "alpha"},
				{secondPID, "beta"},
			} {
				if err := waitForSubjectContains(rt, fmt.Sprintf("repl.subtone.%d", spec.pid), 10*time.Second, []string{
					`"scope":"subtone"`,
					fmt.Sprintf(`--filter %s`, spec.filter),
				}); err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("subtone %d did not retain isolated command payload: %w", spec.pid, err)
				}
			}
			if err := cleanupManagedSubtones(rt); err != nil {
				return testv1.StepRunResult{}, err
			}

			return testv1.StepRunResult{
				Report: "Started two concurrent background watch subtones, verified `/ps` showed both pids, confirmed each subtone room kept its own command payload, then cleaned managed processes before the next step.",
			}, nil
		},
	})
}

func startBackgroundWatch(rt *support.Runtime, filter string) (int, error) {
	prevPID, _ := rt.LatestSubtonePID()
	cmd := fmt.Sprintf("/repl src_v3 watch --nats-url %s --subject repl.room.index --filter %s &", rt.NATSURL, strings.TrimSpace(filter))
	if err := rt.SendJoinLine(cmd); err != nil {
		return 0, err
	}
	if err := rt.WaitForPatterns(20*time.Second, []string{
		fmt.Sprintf(`--filter %s`, strings.TrimSpace(filter)),
		`"scope":"index"`,
		`Request received. Spawning subtone for repl src_v3`,
		`Subtone started as pid `,
		`Subtone room: subtone-`,
		`Subtone log file: `,
		`Subtone for repl src_v3 is running in background.`,
	}); err != nil {
		return 0, fmt.Errorf("background watch subtone did not start cleanly: %w", err)
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
		return 0, fmt.Errorf("no new background subtone pid observed after %q", cmd)
	}
	if err := waitForSubjectContains(rt, fmt.Sprintf("repl.subtone.%d", pid), 10*time.Second, []string{
		`"scope":"subtone"`,
		`watching NATS subject`,
		fmt.Sprintf(`--filter %s`, strings.TrimSpace(filter)),
	}); err != nil {
		return 0, err
	}
	return pid, nil
}

func cleanupManagedSubtones(rt *support.Runtime) error {
	listCmd := "/repl src_v3 subtone-list --count 50"
	if err := rt.RunTranscript([]support.TranscriptStep{{
		Send: listCmd,
		ExpectRoom: support.CombinePatterns([]string{
			fmt.Sprintf(`"message":"%s"`, listCmd),
		}, support.StandardSubtoneRoomPatterns("repl src_v3", "")),
		ExpectOutput: append([]string{fmt.Sprintf("llm-codex> %s", listCmd)}, support.StandardSubtoneOutputPatterns("repl src_v3", "")...),
		Timeout:      30 * time.Second,
	}}); err != nil {
		return fmt.Errorf("repl subtone-list cleanup failed: %w", err)
	}
	listPID, err := rt.WaitForSubtonePIDForCommand("repl src_v3 subtone-list --count 50", 20*time.Second)
	if err != nil {
		return fmt.Errorf("missing cleanup subtone-list pid: %w", err)
	}
	out := strings.Join(rt.SubjectMessages(fmt.Sprintf("repl.subtone.%d", listPID)), "\n")
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
		if len(fields) < 3 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil || pid <= 0 {
			continue
		}
		if pid == listPID {
			continue
		}
		if strings.TrimSpace(fields[2]) != "active" {
			continue
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("find active subtone pid %d: %w", pid, err)
		}
		if err := proc.Kill(); err != nil {
			return fmt.Errorf("kill active subtone pid %d: %w", pid, err)
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
