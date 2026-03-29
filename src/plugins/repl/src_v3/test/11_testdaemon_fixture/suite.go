package testdaemonfixture

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

var queuedTaskIDRe = regexp.MustCompile(`Task queued as (task-[A-Za-z0-9-]+)`)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "testdaemon-builds",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newFixtureRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			run, err := runQueuedCommand(rt, "testdaemon", "src_v1", "build")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			showOut, err := waitForTaskDone(rt, run.TaskID, 45*time.Second)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			for _, expected := range []string{
				"Task: " + run.TaskID,
				"State: done",
				"Command: testdaemon src_v1 build",
				"Exit code: 0",
			} {
				if !strings.Contains(showOut, expected) {
					return testv1.StepRunResult{}, fmt.Errorf("build task show output missing %q\n%s", expected, showOut)
				}
			}

			ctx.TestPassf("testdaemon build completed as task %s", run.TaskID)
			return testv1.StepRunResult{
				Report: "Queued `./dialtone.sh testdaemon src_v1 build` through the shell REPL path, verified the task-first shell transcript, then confirmed `task show` reported the finished build task with exit code 0.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "testdaemon-one-shot-command-emits-progress",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newFixtureRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			run, err := runQueuedCommand(rt, "testdaemon", "src_v1", "emit-progress", "--steps", "3", "--name", "repl-progress")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if _, err := waitForTaskDone(rt, run.TaskID, 45*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			logOut, err := readTaskLog(rt, run.TaskID, 120)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			for _, expected := range []string{
				"Task log:",
				"progress 1/3",
				"progress 2/3",
				"progress 3/3",
				"progress complete",
			} {
				if !strings.Contains(logOut, expected) {
					return testv1.StepRunResult{}, fmt.Errorf("task log for %s missing %q\n%s", run.TaskID, expected, logOut)
				}
			}

			ctx.TestPassf("testdaemon progress command wrote expected task log for %s", run.TaskID)
			return testv1.StepRunResult{
				Report: "Queued `testdaemon src_v1 emit-progress --steps 3`, waited for the task to finish, and verified `task log --task-id` preserved the emitted progress lines in the durable task log.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "testdaemon-can-exit-nonzero",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newFixtureRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			run, err := runQueuedCommand(rt, "testdaemon", "src_v1", "exit-code", "--code", "17")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			showOut, err := waitForTaskShowContains(rt, run.TaskID, 45*time.Second,
				"State: done",
				"Command: testdaemon src_v1 exit-code --code 17",
				"Exit code: 1",
			)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			logOut, err := readTaskLog(rt, run.TaskID, 160)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			for _, expected := range []string{
				"Task log:",
				"testdaemon> exiting with code=17",
				"testdaemon exit-code requested code=17",
				"exit status 17",
				"lifecycle exited pid=",
				"code=1",
			} {
				if !strings.Contains(logOut, expected) {
					return testv1.StepRunResult{}, fmt.Errorf("nonzero task log for %s missing %q\nshow:\n%s\nlog:\n%s", run.TaskID, expected, showOut, logOut)
				}
			}

			ctx.TestPassf("nonzero exit task %s preserved requested code details in the task log", run.TaskID)
			return testv1.StepRunResult{
				Report: "Queued `testdaemon src_v1 exit-code --code 17`, verified the task finished nonzero through `task show`, and confirmed the durable task log preserved both the requested fixture code and the worker-level nonzero lifecycle.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "testdaemon-can-panic",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newFixtureRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			run, err := runQueuedCommand(rt, "testdaemon", "src_v1", "panic")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			showOut, err := waitForTaskShowContains(rt, run.TaskID, 45*time.Second,
				"State: done",
				"Command: testdaemon src_v1 panic",
				"Exit code: 2",
			)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			logOut, err := readTaskLog(rt, run.TaskID, 200)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			for _, expected := range []string{
				"Task log:",
				"testdaemon> panic requested",
				"panic: testdaemon panic requested",
				"lifecycle exited pid=",
				"code=2",
			} {
				if !strings.Contains(logOut, expected) {
					return testv1.StepRunResult{}, fmt.Errorf("panic task log for %s missing %q\nshow:\n%s\nlog:\n%s", run.TaskID, expected, showOut, logOut)
				}
			}

			ctx.TestPassf("panic task %s surfaced a durable task log and failed lifecycle", run.TaskID)
			return testv1.StepRunResult{
				Report: "Queued `testdaemon src_v1 panic`, verified the task failed through `task show`, and confirmed the durable task log retained the panic marker and failing lifecycle details.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "testdaemon-can-hang",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newFixtureRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			run, err := runQueuedCommand(rt, "testdaemon", "src_v1", "hang")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			runningShow, err := waitForTaskShowContains(rt, run.TaskID, 20*time.Second,
				"State: running",
				"Command: testdaemon src_v1 hang",
			)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			runningLog, err := readTaskLog(rt, run.TaskID, 80)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			for _, expected := range []string{
				"Task log:",
				"testdaemon> hang requested",
			} {
				if !strings.Contains(runningLog, expected) {
					return testv1.StepRunResult{}, fmt.Errorf("hang task log for %s missing %q while running\nshow:\n%s\nlog:\n%s", run.TaskID, expected, runningShow, runningLog)
				}
			}

			killOut, err := rt.RunDialtone("repl", "src_v3", "task", "kill", "--task-id", run.TaskID)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("task kill failed for %s: %w\n%s", run.TaskID, err, killOut)
			}
			for _, expected := range []string{
				"Stopping task " + run.TaskID,
				"Stop signal sent to task " + run.TaskID + ".",
			} {
				if !strings.Contains(killOut, expected) {
					return testv1.StepRunResult{}, fmt.Errorf("task kill output for %s missing %q\n%s", run.TaskID, expected, killOut)
				}
			}

			finalShow, err := waitForTaskShowContains(rt, run.TaskID, 20*time.Second,
				"State: done",
				"Exit code: -1",
			)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			finalLog, err := readTaskLog(rt, run.TaskID, 100)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			for _, expected := range []string{
				"lifecycle exited pid=",
				"code=-1",
			} {
				if !strings.Contains(finalLog, expected) {
					return testv1.StepRunResult{}, fmt.Errorf("hang task log for %s missing final %q\nshow:\n%s\nlog:\n%s", run.TaskID, expected, finalShow, finalLog)
				}
			}

			ctx.TestPassf("hang task %s stayed running until task kill completed cleanup", run.TaskID)
			return testv1.StepRunResult{
				Report: "Queued `testdaemon src_v1 hang`, verified the task stayed running with durable log output, then used foreground `task kill` to end it and confirmed `task show` and `task log` recorded the final terminated lifecycle.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "testdaemon-service-starts",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newFixtureRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			serviceName := fixtureServiceName("start")
			defer bestEffortStopService(rt, serviceName)

			run, err := runQueuedCommand(rt, "testdaemon", "src_v1", "service", "--mode", "start", "--name", serviceName, "--heartbeat-interval", "200ms")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if _, err := waitForTaskDone(rt, run.TaskID, 45*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			startSnapshot, startLog, err := statusSnapshotFromTask(rt, run.TaskID)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := requireSnapshotValues(startSnapshot, map[string]string{
				"service":          serviceName,
				"running":          "true",
				"heartbeat_paused": "false",
				"health":           "healthy",
			}, startLog); err != nil {
				return testv1.StepRunResult{}, err
			}

			statusSnapshot, statusLog, err := runServiceStatusSnapshot(rt, serviceName)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := requireSnapshotValues(statusSnapshot, map[string]string{
				"service": serviceName,
				"running": "true",
				"health":  "healthy",
			}, statusLog); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("service %s started healthy through queued control-plane task %s", serviceName, run.TaskID)
			return testv1.StepRunResult{
				Report: "Queued `testdaemon src_v1 service --mode start`, verified the task-first shell transcript, then confirmed both the start task log and a later queued `service --mode status` task reported the service as running and healthy.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "testdaemon-heartbeats-while-running",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newFixtureRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			serviceName := fixtureServiceName("heartbeat")
			defer bestEffortStopService(rt, serviceName)

			run, err := runQueuedCommand(rt, "testdaemon", "src_v1", "service", "--mode", "start", "--name", serviceName, "--heartbeat-interval", "200ms")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if _, err := waitForTaskDone(rt, run.TaskID, 45*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}

			firstSnapshot, firstLog, err := runServiceStatusSnapshot(rt, serviceName)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			firstHeartbeat := strings.TrimSpace(firstSnapshot["last_heartbeat"])
			if err := requireSnapshotValues(firstSnapshot, map[string]string{
				"running": "true",
				"health":  "healthy",
			}, firstLog); err != nil {
				return testv1.StepRunResult{}, err
			}
			if firstHeartbeat == "" {
				return testv1.StepRunResult{}, fmt.Errorf("first status snapshot missing last_heartbeat\n%s", firstLog)
			}

			time.Sleep(900 * time.Millisecond)

			secondSnapshot, secondLog, err := runServiceStatusSnapshot(rt, serviceName)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			secondHeartbeat := strings.TrimSpace(secondSnapshot["last_heartbeat"])
			if err := requireSnapshotValues(secondSnapshot, map[string]string{
				"running": "true",
				"health":  "healthy",
			}, secondLog); err != nil {
				return testv1.StepRunResult{}, err
			}
			if secondHeartbeat == "" || secondHeartbeat == firstHeartbeat {
				return testv1.StepRunResult{}, fmt.Errorf("heartbeat did not advance while service was running\nfirst=%s\nsecond=%s\n%s", firstHeartbeat, secondHeartbeat, secondLog)
			}

			firstAt, firstErr := parseStatusTimestamp(firstHeartbeat)
			secondAt, secondErr := parseStatusTimestamp(secondHeartbeat)
			if firstErr != nil || secondErr != nil || !secondAt.After(firstAt) {
				return testv1.StepRunResult{}, fmt.Errorf("heartbeat timestamps did not move forward\nfirst=%s err=%v\nsecond=%s err=%v", firstHeartbeat, firstErr, secondHeartbeat, secondErr)
			}

			ctx.TestPassf("service %s heartbeat advanced from %s to %s", serviceName, firstHeartbeat, secondHeartbeat)
			return testv1.StepRunResult{
				Report: "Started a `testdaemon` service with a short heartbeat interval, queried queued `service --mode status` twice, and verified the reported `last_heartbeat` advanced while the service remained healthy and running.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "testdaemon-can-stop-heartbeats-without-exiting",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newFixtureRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			serviceName := fixtureServiceName("pause")
			defer bestEffortStopService(rt, serviceName)

			run, err := runQueuedCommand(rt, "testdaemon", "src_v1", "service", "--mode", "start", "--name", serviceName, "--heartbeat-interval", "200ms")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if _, err := waitForTaskDone(rt, run.TaskID, 45*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}

			pauseRun, err := runQueuedCommand(rt, "testdaemon", "src_v1", "heartbeat", "--name", serviceName, "--mode", "stop", "--timeout", "5s")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if _, err := waitForTaskDone(rt, pauseRun.TaskID, 45*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			pauseSnapshot, pauseLog, err := statusSnapshotFromTask(rt, pauseRun.TaskID)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := requireSnapshotValues(pauseSnapshot, map[string]string{
				"service":          serviceName,
				"heartbeat_paused": "true",
				"health":           "paused",
			}, pauseLog); err != nil {
				return testv1.StepRunResult{}, err
			}

			statusSnapshot, statusLog, err := runServiceStatusSnapshot(rt, serviceName)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := requireSnapshotValues(statusSnapshot, map[string]string{
				"service":          serviceName,
				"running":          "true",
				"heartbeat_paused": "true",
				"health":           "paused",
			}, statusLog); err != nil {
				return testv1.StepRunResult{}, err
			}

			resumeRun, err := runQueuedCommand(rt, "testdaemon", "src_v1", "heartbeat", "--name", serviceName, "--mode", "resume", "--timeout", "5s")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if _, err := waitForTaskDone(rt, resumeRun.TaskID, 45*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			resumeSnapshot, resumeLog, err := statusSnapshotFromTask(rt, resumeRun.TaskID)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := requireSnapshotValues(resumeSnapshot, map[string]string{
				"service":          serviceName,
				"heartbeat_paused": "false",
				"health":           "healthy",
			}, resumeLog); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("service %s paused heartbeats without exiting and later resumed healthy", serviceName)
			return testv1.StepRunResult{
				Report: "Started a `testdaemon` service, queued `heartbeat --mode stop`, verified a later queued `service --mode status` still showed the service running while heartbeats were paused, then queued `heartbeat --mode resume` and confirmed the service returned to healthy heartbeats.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "testdaemon-service-stops",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newFixtureRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			serviceName := fixtureServiceName("stop")

			run, err := runQueuedCommand(rt, "testdaemon", "src_v1", "service", "--mode", "start", "--name", serviceName, "--heartbeat-interval", "200ms")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if _, err := waitForTaskDone(rt, run.TaskID, 45*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}

			stopRun, err := runQueuedCommand(rt, "testdaemon", "src_v1", "service", "--mode", "stop", "--name", serviceName, "--timeout", "10s")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if _, err := waitForTaskDone(rt, stopRun.TaskID, 45*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			stopSnapshot, stopLog, err := statusSnapshotFromTask(rt, stopRun.TaskID)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := requireSnapshotValues(stopSnapshot, map[string]string{
				"service": serviceName,
				"running": "false",
				"health":  "stopped",
			}, stopLog); err != nil {
				return testv1.StepRunResult{}, err
			}

			statusSnapshot, statusLog, err := runServiceStatusSnapshot(rt, serviceName)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := requireSnapshotValues(statusSnapshot, map[string]string{
				"service": serviceName,
				"running": "false",
				"health":  "stopped",
			}, statusLog); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("service %s stopped cleanly through queued task %s", serviceName, stopRun.TaskID)
			return testv1.StepRunResult{
				Report: "Queued `testdaemon src_v1 service --mode stop`, verified the stop task completed, and confirmed both the stop task log and a later queued `service --mode status` snapshot reported the service as stopped.",
			}, nil
		},
	})
}

type queuedCommandResult struct {
	TaskID      string
	LogPath     string
	ShellOutput string
}

func newFixtureRuntime(ctx *testv1.StepContext) (*support.Runtime, error) {
	rt, err := support.NewIsolatedRuntime(ctx)
	if err != nil {
		return nil, err
	}
	rt.NATSURL = allocateLocalNATSURL()
	if _, cleanErr := rt.RunDialtone("repl", "src_v3", "process-clean"); cleanErr != nil {
		// Best-effort cleanup only; new NATS URL isolates the actual test path.
	}
	return rt, nil
}

func runQueuedCommand(rt *support.Runtime, args ...string) (queuedCommandResult, error) {
	out, err := rt.RunDialtone(args...)
	if err != nil {
		return queuedCommandResult{}, fmt.Errorf("queued command %q failed: %w\n%s", strings.Join(args, " "), err, out)
	}
	if err := support.MatchAnyPatternGroupText(out, [][]string{support.StandardShellTaskOutputPatterns()}); err != nil {
		return queuedCommandResult{}, fmt.Errorf("queued command %q missed task-first shell contract: %w\n%s", strings.Join(args, " "), err, out)
	}
	taskID, err := parseQueuedTaskID(out)
	if err != nil {
		return queuedCommandResult{}, err
	}
	logPath, err := support.ParseWorkLogPath(out)
	if err != nil {
		return queuedCommandResult{}, err
	}
	return queuedCommandResult{
		TaskID:      taskID,
		LogPath:     logPath,
		ShellOutput: out,
	}, nil
}

func parseQueuedTaskID(out string) (string, error) {
	match := queuedTaskIDRe.FindStringSubmatch(out)
	if len(match) != 2 {
		return "", fmt.Errorf("task id not found in shell output\n%s", out)
	}
	return strings.TrimSpace(match[1]), nil
}

func waitForTaskDone(rt *support.Runtime, taskID string, timeout time.Duration) (string, error) {
	return waitForTaskShowContains(rt, taskID, timeout, "State: done")
}

func waitForTaskShowContains(rt *support.Runtime, taskID string, timeout time.Duration, patterns ...string) (string, error) {
	deadline := time.Now().Add(timeout)
	lastOut := ""
	for time.Now().Before(deadline) {
		out, err := rt.RunDialtone("repl", "src_v3", "task", "show", "--task-id", taskID)
		if err == nil {
			lastOut = out
			matched := strings.Contains(out, "Task: "+taskID)
			for _, pattern := range patterns {
				if !strings.Contains(out, pattern) {
					matched = false
					break
				}
			}
			if matched {
				return out, nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return lastOut, fmt.Errorf("timed out waiting for task %s to contain %s\n%s", taskID, strings.Join(patterns, ", "), lastOut)
}

func readTaskLog(rt *support.Runtime, taskID string, lines int) (string, error) {
	out, err := rt.RunDialtone("repl", "src_v3", "task", "log", "--task-id", taskID, "--lines", strconv.Itoa(lines))
	if err != nil {
		return out, fmt.Errorf("task log failed for %s: %w\n%s", taskID, err, out)
	}
	return out, nil
}

func statusSnapshotFromTask(rt *support.Runtime, taskID string) (map[string]string, string, error) {
	logOut, err := readTaskLog(rt, taskID, 200)
	if err != nil {
		return nil, logOut, err
	}
	return parseFixtureSnapshot(logOut), logOut, nil
}

func runServiceStatusSnapshot(rt *support.Runtime, serviceName string) (map[string]string, string, error) {
	run, err := runQueuedCommand(rt, "testdaemon", "src_v1", "service", "--mode", "status", "--name", serviceName)
	if err != nil {
		return nil, "", err
	}
	if _, err := waitForTaskDone(rt, run.TaskID, 45*time.Second); err != nil {
		return nil, "", err
	}
	return statusSnapshotFromTask(rt, run.TaskID)
}

func parseFixtureSnapshot(body string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "testdaemon> ") {
			continue
		}
		idx := strings.Index(line, "testdaemon> ")
		payload := strings.TrimSpace(line[idx+len("testdaemon> "):])
		key, value, ok := strings.Cut(payload, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return out
}

func requireSnapshotValues(snapshot map[string]string, expected map[string]string, body string) error {
	for key, want := range expected {
		got := strings.TrimSpace(snapshot[strings.TrimSpace(key)])
		if got != strings.TrimSpace(want) {
			return fmt.Errorf("snapshot missing %s=%s (got %q)\n%s", key, want, got, body)
		}
	}
	return nil
}

func bestEffortStopService(rt *support.Runtime, serviceName string) {
	run, err := runQueuedCommand(rt, "testdaemon", "src_v1", "service", "--mode", "stop", "--name", serviceName, "--timeout", "10s")
	if err != nil {
		return
	}
	_, _ = waitForTaskDone(rt, run.TaskID, 30*time.Second)
}

func fixtureServiceName(prefix string) string {
	return fmt.Sprintf("repl-fixture-%s-%d", strings.TrimSpace(prefix), time.Now().UnixNano())
}

func parseStatusTimestamp(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q", raw)
}

func allocateLocalNATSURL() string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "nats://127.0.0.1:46322"
	}
	defer ln.Close()
	return "nats://" + strings.TrimSpace(ln.Addr().String())
}
