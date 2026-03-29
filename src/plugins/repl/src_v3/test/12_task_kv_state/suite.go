package taskkvstate

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"dialtone/dev/plugins/proc/src_v1/go/proc"
	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/nats-io/nats.go"
)

const taskKVBucketName = "repl_task_v3"

var queuedTaskIDRe = regexp.MustCompile(`Task queued as (task-[A-Za-z0-9-]+)`)

type taskKVRecord struct {
	TaskID    string   `json:"task_id,omitempty"`
	Command   string   `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
	Topic     string   `json:"topic,omitempty"`
	LogPath   string   `json:"log_path,omitempty"`
	Host      string   `json:"host,omitempty"`
	Mode      string   `json:"mode,omitempty"`
	State     string   `json:"state,omitempty"`
	PID       int      `json:"pid,omitempty"`
	ExitCode  *int     `json:"exit_code,omitempty"`
	CreatedAt string   `json:"created_at,omitempty"`
	UpdatedAt string   `json:"updated_at,omitempty"`
	StartedAt string   `json:"started_at,omitempty"`
	LastOKAt  string   `json:"last_ok_at,omitempty"`
	Service   string   `json:"service,omitempty"`
	WorkerLog string   `json:"worker_log,omitempty"`
}

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "task-submit-creates-kv-record-before-launch",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newTaskKVRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			holdPath := filepath.Join(os.TempDir(), fmt.Sprintf("dialtone-task-kv-hold-%d", time.Now().UnixNano()))
			if err := os.WriteFile(holdPath, []byte("hold"), 0o644); err != nil {
				return testv1.StepRunResult{}, err
			}
			defer os.Remove(holdPath)

			prev := strings.TrimSpace(os.Getenv("DIALTONE_REPL_TEST_TASK_START_HOLD_FILE"))
			if err := os.Setenv("DIALTONE_REPL_TEST_TASK_START_HOLD_FILE", holdPath); err != nil {
				return testv1.StepRunResult{}, err
			}
			defer restoreEnv("DIALTONE_REPL_TEST_TASK_START_HOLD_FILE", prev)

			out, err := rt.RunDialtone("proc", "src_v1", "emit", "task-kv-prelaunch")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("queued command failed: %w\n%s", err, out)
			}
			if err := support.MatchAnyPatternGroupText(out, [][]string{support.StandardShellTaskOutputPatterns()}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("queued shell output missed task-first contract: %w\n%s", err, out)
			}
			taskID, err := parseQueuedTaskID(out)
			if err != nil {
				return testv1.StepRunResult{}, err
			}

			record, err := waitForTaskKVRecord(rt.NATSURL, taskID, 15*time.Second, func(rec taskKVRecord) bool {
				return strings.EqualFold(strings.TrimSpace(rec.State), "queued") && rec.PID == 0
			})
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if record.TaskID != taskID {
				return testv1.StepRunResult{}, fmt.Errorf("kv record task id mismatch: want=%s got=%s", taskID, record.TaskID)
			}

			if err := os.Remove(holdPath); err != nil && !os.IsNotExist(err) {
				return testv1.StepRunResult{}, err
			}
			if _, err := waitForTaskShowContains(rt, taskID, 30*time.Second, "State: done"); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("task %s wrote queued KV state before process launch", taskID)
			return testv1.StepRunResult{
				Report: "Queued a shell-routed task while a test-only prelaunch hold file was active, inspected the NATS KV task record directly, and verified the record existed in `queued` state before any PID assignment occurred.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "task-record-includes-task-id-command-topic-log-host",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newTaskKVRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			out, err := rt.RunDialtone("proc", "src_v1", "emit", "task-kv-fields")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("queued command failed: %w\n%s", err, out)
			}
			taskID, err := parseQueuedTaskID(out)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			record, err := waitForTaskKVRecord(rt.NATSURL, taskID, 20*time.Second, func(rec taskKVRecord) bool {
				return strings.TrimSpace(rec.TaskID) != "" &&
					strings.TrimSpace(rec.Command) != "" &&
					strings.TrimSpace(rec.Topic) != "" &&
					strings.TrimSpace(rec.LogPath) != "" &&
					strings.TrimSpace(rec.Host) != "" &&
					strings.TrimSpace(rec.CreatedAt) != "" &&
					strings.TrimSpace(rec.UpdatedAt) != ""
			})
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if record.TaskID != taskID {
				return testv1.StepRunResult{}, fmt.Errorf("task record task id mismatch: want=%s got=%s", taskID, record.TaskID)
			}
			if strings.TrimSpace(record.Command) != "proc src_v1 emit task-kv-fields" {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected task record command: %q", record.Command)
			}
			if want := "task." + taskID; strings.TrimSpace(record.Topic) != want {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected task topic: want=%s got=%s", want, record.Topic)
			}
			if !strings.Contains(strings.TrimSpace(record.LogPath), taskID) {
				return testv1.StepRunResult{}, fmt.Errorf("task log path does not include task id %s: %s", taskID, record.LogPath)
			}
			if !strings.HasPrefix(filepath.Base(strings.TrimSpace(record.LogPath)), taskID) {
				return testv1.StepRunResult{}, fmt.Errorf("task log base name is not task-first: %s", record.LogPath)
			}
			if len(record.Args) != 4 {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected args length: %d %#v", len(record.Args), record.Args)
			}

			ctx.TestPassf("task %s stored canonical fields in KV", taskID)
			return testv1.StepRunResult{
				Report: "Queued a one-shot task and read its NATS KV record directly to confirm the canonical task identity fields were present: task id, command, args, topic, log path, host, and timestamps.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "task-record-exists-before-pid-assignment",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newTaskKVRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			holdPath := filepath.Join(os.TempDir(), fmt.Sprintf("dialtone-task-kv-pid-hold-%d", time.Now().UnixNano()))
			if err := os.WriteFile(holdPath, []byte("hold"), 0o644); err != nil {
				return testv1.StepRunResult{}, err
			}
			defer os.Remove(holdPath)

			prev := strings.TrimSpace(os.Getenv("DIALTONE_REPL_TEST_TASK_START_HOLD_FILE"))
			if err := os.Setenv("DIALTONE_REPL_TEST_TASK_START_HOLD_FILE", holdPath); err != nil {
				return testv1.StepRunResult{}, err
			}
			defer restoreEnv("DIALTONE_REPL_TEST_TASK_START_HOLD_FILE", prev)

			out, err := rt.RunDialtone("proc", "src_v1", "emit", "task-kv-no-pid-yet")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("queued command failed: %w\n%s", err, out)
			}
			taskID, err := parseQueuedTaskID(out)
			if err != nil {
				return testv1.StepRunResult{}, err
			}

			record, err := waitForTaskKVRecord(rt.NATSURL, taskID, 15*time.Second, func(rec taskKVRecord) bool {
				return strings.EqualFold(strings.TrimSpace(rec.State), "queued")
			})
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if record.PID != 0 {
				return testv1.StepRunResult{}, fmt.Errorf("expected queued kv record before pid assignment, got pid=%d", record.PID)
			}
			if record.StartedAt != "" {
				return testv1.StepRunResult{}, fmt.Errorf("expected empty started_at before pid assignment, got %s", record.StartedAt)
			}
			if record.WorkerLog != "" {
				return testv1.StepRunResult{}, fmt.Errorf("expected empty worker log before pid assignment, got %s", record.WorkerLog)
			}

			if err := os.Remove(holdPath); err != nil && !os.IsNotExist(err) {
				return testv1.StepRunResult{}, err
			}
			if _, err := waitForTaskKVRecord(rt.NATSURL, taskID, 20*time.Second, func(rec taskKVRecord) bool {
				return strings.EqualFold(strings.TrimSpace(rec.State), "running") && rec.PID > 0
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if _, err := waitForTaskShowContains(rt, taskID, 30*time.Second, "State: done"); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("task %s stayed visible in KV before PID assignment and then advanced to running", taskID)
			return testv1.StepRunResult{
				Report: "Held task launch behind a test-only prestart gate, verified the NATS KV record already existed without a PID or worker-start metadata, then released the hold and confirmed the same task id advanced to a running record.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "task-record-updates-to-running-after-launch",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newTaskKVRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			out, err := rt.RunDialtone("testdaemon", "src_v1", "hang")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("queued command failed: %w\n%s", err, out)
			}
			taskID, err := parseQueuedTaskID(out)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			record, err := waitForTaskKVRecord(rt.NATSURL, taskID, 20*time.Second, func(rec taskKVRecord) bool {
				return strings.EqualFold(strings.TrimSpace(rec.State), "running")
			})
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if record.PID <= 0 {
				return testv1.StepRunResult{}, fmt.Errorf("expected running task record to have pid, got %+v", record)
			}

			if err := stopTask(rt, taskID, 20*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("task %s advanced to running in KV after launch", taskID)
			return testv1.StepRunResult{
				Report: "Queued a long-running task, read the NATS KV record directly, and verified the same task id moved from its queued entry to `running` state after launch.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "task-record-stores-pid-after-launch",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newTaskKVRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			out, err := rt.RunDialtone("testdaemon", "src_v1", "hang")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("queued command failed: %w\n%s", err, out)
			}
			taskID, err := parseQueuedTaskID(out)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			record, err := waitForTaskKVRecord(rt.NATSURL, taskID, 20*time.Second, func(rec taskKVRecord) bool {
				return rec.PID > 0 && strings.EqualFold(strings.TrimSpace(rec.State), "running")
			})
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			showOut, err := waitForTaskShowContains(rt, taskID, 20*time.Second, fmt.Sprintf("PID: %d", record.PID))
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if !strings.Contains(showOut, "State: running") {
				return testv1.StepRunResult{}, fmt.Errorf("task show for %s did not reflect running pid %d\n%s", taskID, record.PID, showOut)
			}

			if err := stopTask(rt, taskID, 20*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("task %s stored running pid %d in KV", taskID, record.PID)
			return testv1.StepRunResult{
				Report: "Queued a long-running task, confirmed the NATS KV record stored the assigned PID after launch, and verified `task show` reflected that same PID while the task was still running.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "task-record-updates-to-exited-on-finish",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newTaskKVRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			out, err := rt.RunDialtone("proc", "src_v1", "emit", "task-kv-finished")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("queued command failed: %w\n%s", err, out)
			}
			taskID, err := parseQueuedTaskID(out)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			record, err := waitForTaskKVRecord(rt.NATSURL, taskID, 20*time.Second, func(rec taskKVRecord) bool {
				return strings.EqualFold(strings.TrimSpace(rec.State), "done")
			})
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if record.UpdatedAt == "" {
				return testv1.StepRunResult{}, fmt.Errorf("expected done task record to have updated_at, got %+v", record)
			}
			if _, err := waitForTaskShowContains(rt, taskID, 20*time.Second, "State: done", "Exit code: 0"); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("task %s advanced to done in KV after finish", taskID)
			return testv1.StepRunResult{
				Report: "Queued a short-lived task, read the NATS KV record directly, and verified the same task id advanced to `done` once the worker finished successfully.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "task-record-stores-exit-code-on-finish",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newTaskKVRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			out, err := rt.RunDialtone("testdaemon", "src_v1", "exit-code", "--code", "17")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("queued command failed before task submission: %w\n%s", err, out)
			}
			taskID, err := parseQueuedTaskID(out)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			record, err := waitForTaskKVRecord(rt.NATSURL, taskID, 20*time.Second, func(rec taskKVRecord) bool {
				return strings.EqualFold(strings.TrimSpace(rec.State), "done") && rec.ExitCode != nil
			})
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if record.ExitCode == nil || *record.ExitCode != 17 {
				return testv1.StepRunResult{}, fmt.Errorf("expected task %s exit code 17 in KV, got %+v", taskID, record)
			}
			if _, err := waitForTaskShowContains(rt, taskID, 20*time.Second, "State: done", "Exit code: 17"); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("task %s stored exit code 17 in KV after finish", taskID)
			return testv1.StepRunResult{
				Report: "Queued `testdaemon src_v1 exit-code --code 17`, confirmed the shell returned a nonzero task outcome, then read the NATS KV task record directly and verified the finished task persisted exit code `17`.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "task-list-reads-from-kv",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newTaskKVRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			if err := rt.StartLeader(); err != nil {
				return testv1.StepRunResult{}, err
			}
			taskID := fmt.Sprintf("task-kv-list-%d", time.Now().UnixNano())
			record := syntheticTaskKVRecord(taskID, "kv synthetic queued list proof", "queued")
			if err := putTaskKVRecord(rt.NATSURL, record); err != nil {
				return testv1.StepRunResult{}, err
			}
			listOut, err := waitForTaskListContains(rt, "queued", 20*time.Second, taskID, "queued", record.Command)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if strings.Contains(listOut, "No queued tasks reported by leader.") {
				return testv1.StepRunResult{}, fmt.Errorf("task list did not surface synthetic KV task %s\n%s", taskID, listOut)
			}

			ctx.TestPassf("task list surfaced synthetic queued record %s from KV", taskID)
			return testv1.StepRunResult{
				Report: "Started a leader, inserted a synthetic queued task record directly into the NATS KV bucket without creating any process or log file, and verified `task list --state queued` surfaced that record through the operator CLI.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "task-show-reads-from-kv",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newTaskKVRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			if err := rt.StartLeader(); err != nil {
				return testv1.StepRunResult{}, err
			}
			taskID := fmt.Sprintf("task-kv-show-%d", time.Now().UnixNano())
			record := syntheticTaskKVRecord(taskID, "kv synthetic show proof", "queued")
			record.Host = "kv-show-host"
			record.Mode = "background"
			if err := putTaskKVRecord(rt.NATSURL, record); err != nil {
				return testv1.StepRunResult{}, err
			}
			if _, err := waitForTaskShowContains(rt, taskID, 20*time.Second,
				"State: queued",
				"Host: kv-show-host",
				"Mode: background",
				"Topic: task."+taskID,
				"Command: kv synthetic show proof",
			); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("task show surfaced synthetic queued record %s from KV", taskID)
			return testv1.StepRunResult{
				Report: "Started a leader, inserted a synthetic queued task record directly into the NATS KV bucket without a backing task log, and verified `task show --task-id` returned the KV-backed snapshot fields instead of falling back to log scanning.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "task-show-prefers-kv-over-live-registry-fields",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			prevHeartbeatSec := strings.TrimSpace(os.Getenv("DIALTONE_SUBTONE_HEARTBEAT_SEC"))
			if err := os.Setenv("DIALTONE_SUBTONE_HEARTBEAT_SEC", "60"); err != nil {
				return testv1.StepRunResult{}, err
			}
			defer restoreEnv("DIALTONE_SUBTONE_HEARTBEAT_SEC", prevHeartbeatSec)

			rt, err := newTaskKVRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()
			livePID := 0
			defer func() {
				bestEffortKillManagedProcess(livePID)
			}()

			out, err := rt.RunDialtone("testdaemon", "src_v1", "hang")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("queued command failed: %w\n%s", err, out)
			}
			taskID, err := parseQueuedTaskID(out)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			record, err := waitForTaskKVRecord(rt.NATSURL, taskID, 20*time.Second, func(rec taskKVRecord) bool {
				return strings.EqualFold(strings.TrimSpace(rec.State), "running") && rec.PID > 0
			})
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			livePID = record.PID

			mutated := record
			mutated.Host = "kv-override-host"
			mutated.Mode = "background"
			mutated.Command = "kv override show proof"
			mutated.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err := putTaskKVRecord(rt.NATSURL, mutated); err != nil {
				return testv1.StepRunResult{}, err
			}

			showOut, err := waitForTaskShowContains(rt, taskID, 10*time.Second,
				"Host: kv-override-host",
				"Mode: background",
				"Command: kv override show proof",
			)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if strings.Contains(showOut, "Command: testdaemon src_v1 hang") {
				return testv1.StepRunResult{}, fmt.Errorf("task show still surfaced the live registry command instead of the KV override\n%s", showOut)
			}

			ctx.TestPassf("task show followed KV overrides for live task %s instead of the in-memory registry", taskID)
			return testv1.StepRunResult{
				Report: "Queued a long-running task, waited for the live registry to see it as running, then overwrote the same task id inside NATS KV with different host, mode, and command fields. `task show` returned the KV-mutated values while the process was still alive, proving the operator query follows KV rather than leader memory.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "task-list-prefers-kv-over-live-registry-fields",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			prevHeartbeatSec := strings.TrimSpace(os.Getenv("DIALTONE_SUBTONE_HEARTBEAT_SEC"))
			if err := os.Setenv("DIALTONE_SUBTONE_HEARTBEAT_SEC", "60"); err != nil {
				return testv1.StepRunResult{}, err
			}
			defer restoreEnv("DIALTONE_SUBTONE_HEARTBEAT_SEC", prevHeartbeatSec)

			rt, err := newTaskKVRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()
			livePID := 0
			defer func() {
				bestEffortKillManagedProcess(livePID)
			}()

			out, err := rt.RunDialtone("testdaemon", "src_v1", "hang")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("queued command failed: %w\n%s", err, out)
			}
			taskID, err := parseQueuedTaskID(out)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			record, err := waitForTaskKVRecord(rt.NATSURL, taskID, 20*time.Second, func(rec taskKVRecord) bool {
				return strings.EqualFold(strings.TrimSpace(rec.State), "running") && rec.PID > 0
			})
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			livePID = record.PID

			mutated := record
			mutated.Mode = "background"
			mutated.Command = "kv override list proof"
			mutated.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err := putTaskKVRecord(rt.NATSURL, mutated); err != nil {
				return testv1.StepRunResult{}, err
			}

			listOut, err := waitForTaskListContains(rt, "all", 10*time.Second, taskID, "background", "kv override list proof")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if strings.Contains(listOut, "testdaemon src_v1 hang") {
				return testv1.StepRunResult{}, fmt.Errorf("task list still surfaced the live registry command instead of the KV override\n%s", listOut)
			}

			ctx.TestPassf("task list followed KV overrides for live task %s instead of the in-memory registry", taskID)
			return testv1.StepRunResult{
				Report: "Queued a long-running task, overwrote its command and mode inside NATS KV while the process was still running, and verified `task list --state running` rendered the KV-mutated values rather than the leader's live registry snapshot.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "task-state-queries-follow-kv-over-finished-task-history",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newTaskKVRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			out, err := rt.RunDialtone("testdaemon", "src_v1", "exit-code", "--code", "17")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("queued command failed before task submission: %w\n%s", err, out)
			}
			taskID, err := parseQueuedTaskID(out)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			record, err := waitForTaskKVRecord(rt.NATSURL, taskID, 20*time.Second, func(rec taskKVRecord) bool {
				return strings.EqualFold(strings.TrimSpace(rec.State), "done") && rec.ExitCode != nil
			})
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if record.ExitCode == nil || *record.ExitCode != 17 {
				return testv1.StepRunResult{}, fmt.Errorf("expected finished task %s exit code 17 before KV override, got %+v", taskID, record)
			}

			mutated := record
			mutated.State = "running"
			mutated.ExitCode = nil
			mutated.Host = "kv-state-host"
			mutated.Mode = "background"
			mutated.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err := putTaskKVRecord(rt.NATSURL, mutated); err != nil {
				return testv1.StepRunResult{}, err
			}

			if _, err := waitForTaskShowContains(rt, taskID, 10*time.Second,
				"Host: kv-state-host",
				"State: running",
				"Mode: background",
			); err != nil {
				return testv1.StepRunResult{}, err
			}
			if _, err := waitForTaskListContains(rt, "running", 10*time.Second, taskID, "running"); err != nil {
				return testv1.StepRunResult{}, err
			}
			allOut, err := waitForTaskLogContains(rt, taskID, 10*time.Second, "exit-code requested code=17")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("%w\n%s", err, allOut)
			}

			ctx.TestPassf("task queries followed KV state for finished task %s instead of the durable task history", taskID)
			return testv1.StepRunResult{
				Report: "Queued `testdaemon src_v1 exit-code --code 17`, waited for the finished task record, then overwrote the same task id in NATS KV to look `running` again. `task show` and `task list --state running` followed the KV override even though the durable task log still recorded the original finished failure, proving the query surface follows KV state rather than reconstructing from task history.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "task-log-by-task-id-still-works-after-task-exit",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newTaskKVRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			out, err := rt.RunDialtone("testdaemon", "src_v1", "emit-progress", "--steps", "3", "--name", "task-kv-log")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("queued command failed: %w\n%s", err, out)
			}
			taskID, err := parseQueuedTaskID(out)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if _, err := waitForTaskShowContains(rt, taskID, 20*time.Second, "State: done", "Exit code: 0"); err != nil {
				return testv1.StepRunResult{}, err
			}
			logOut, err := waitForTaskLogContains(rt, taskID, 20*time.Second,
				"Task log:",
				"progress 1/3",
				"progress 2/3",
				"progress 3/3",
				"progress complete",
			)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if !strings.Contains(logOut, taskID) {
				return testv1.StepRunResult{}, fmt.Errorf("task log output for %s did not include task id\n%s", taskID, logOut)
			}

			ctx.TestPassf("task log remained readable by task id after task %s exited", taskID)
			return testv1.StepRunResult{
				Report: "Queued `testdaemon src_v1 emit-progress --steps 3`, waited for the task to finish, and verified `task log --task-id` still returned the durable progress output after the worker had exited.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "queued-task-still-visible-after-leader-restart",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newTaskKVRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			if err := rt.StartLeader(); err != nil {
				return testv1.StepRunResult{}, err
			}
			taskID := fmt.Sprintf("task-kv-restart-queued-%d", time.Now().UnixNano())
			record := syntheticTaskKVRecord(taskID, "kv synthetic queued restart proof", "queued")
			record.Host = "restart-host"
			record.Mode = "background"
			if err := putTaskKVRecord(rt.NATSURL, record); err != nil {
				return testv1.StepRunResult{}, err
			}
			if _, err := waitForTaskShowContains(rt, taskID, 20*time.Second, "State: queued"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := runProcessClean(rt); err != nil {
				return testv1.StepRunResult{}, err
			}
			if _, err := waitForTaskShowContains(rt, taskID, 30*time.Second,
				"State: queued",
				"Host: restart-host",
				"Mode: background",
				"Command: kv synthetic queued restart proof",
			); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("synthetic queued task %s survived leader restart through KV", taskID)
			return testv1.StepRunResult{
				Report: "Inserted a synthetic queued task directly into the NATS KV bucket, confirmed it was visible, restarted the leader with `process-clean`, and verified the same queued task was still returned after the new leader came back.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "finished-task-still-visible-after-leader-restart",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := newTaskKVRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			out, err := rt.RunDialtone("testdaemon", "src_v1", "exit-code", "--code", "23")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("queued command failed before task submission: %w\n%s", err, out)
			}
			taskID, err := parseQueuedTaskID(out)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			record, err := waitForTaskKVRecord(rt.NATSURL, taskID, 20*time.Second, func(rec taskKVRecord) bool {
				return strings.EqualFold(strings.TrimSpace(rec.State), "done") && rec.ExitCode != nil
			})
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if record.ExitCode == nil || *record.ExitCode != 23 {
				return testv1.StepRunResult{}, fmt.Errorf("expected task %s exit code 23 before restart, got %+v", taskID, record)
			}
			if err := runProcessClean(rt); err != nil {
				return testv1.StepRunResult{}, err
			}
			if _, err := waitForTaskShowContains(rt, taskID, 30*time.Second,
				"State: done",
				"Exit code: 23",
				"Topic: task."+taskID,
				"Command: testdaemon src_v1 exit-code --code 23",
			); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("finished task %s remained visible with exit code after leader restart", taskID)
			return testv1.StepRunResult{
				Report: "Queued `testdaemon src_v1 exit-code --code 23`, waited for the finished KV record, restarted the leader with `process-clean`, and verified `task show` still returned the same durable task identity and exit code from KV afterward.",
			}, nil
		},
	})
}

func newTaskKVRuntime(ctx *testv1.StepContext) (*support.Runtime, error) {
	rt, err := support.NewIsolatedRuntime(ctx)
	if err != nil {
		return nil, err
	}
	rt.NATSURL = allocateLocalNATSURL()
	return rt, nil
}

func restoreEnv(key string, prev string) {
	if strings.TrimSpace(prev) == "" {
		_ = os.Unsetenv(key)
		return
	}
	_ = os.Setenv(key, prev)
}

func parseQueuedTaskID(out string) (string, error) {
	match := queuedTaskIDRe.FindStringSubmatch(out)
	if len(match) != 2 {
		return "", fmt.Errorf("task id not found in shell output\n%s", out)
	}
	return strings.TrimSpace(match[1]), nil
}

func taskKVKey(taskID string) string {
	return strings.TrimSpace(taskID)
}

func waitForTaskKVRecord(natsURL string, taskID string, timeout time.Duration, predicate func(taskKVRecord) bool) (taskKVRecord, error) {
	deadline := time.Now().Add(timeout)
	last := taskKVRecord{}
	lastErr := error(nil)
	for time.Now().Before(deadline) {
		record, err := readTaskKVRecord(natsURL, taskID)
		if err == nil {
			last = record
			if predicate == nil || predicate(record) {
				return record, nil
			}
		} else {
			lastErr = err
		}
		time.Sleep(200 * time.Millisecond)
	}
	if last.TaskID != "" {
		return last, fmt.Errorf("timed out waiting for task kv record %s to match predicate: %+v", taskID, last)
	}
	return last, fmt.Errorf("timed out waiting for task kv record %s: %v", taskID, lastErr)
}

func readTaskKVRecord(natsURL string, taskID string) (taskKVRecord, error) {
	var record taskKVRecord
	nc, err := nats.Connect(strings.TrimSpace(natsURL), nats.Timeout(1200*time.Millisecond))
	if err != nil {
		return record, err
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return record, err
	}
	kv, err := js.KeyValue(taskKVBucketName)
	if err != nil {
		return record, err
	}
	entry, err := kv.Get(taskKVKey(taskID))
	if err != nil {
		return record, err
	}
	if err := json.Unmarshal(entry.Value(), &record); err != nil {
		return record, err
	}
	return record, nil
}

func putTaskKVRecord(natsURL string, record taskKVRecord) error {
	nc, err := nats.Connect(strings.TrimSpace(natsURL), nats.Timeout(1200*time.Millisecond))
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	kv, err := js.KeyValue(taskKVBucketName)
	if err != nil {
		kv, err = js.CreateKeyValue(&nats.KeyValueConfig{
			Bucket:      taskKVBucketName,
			Description: "REPL task state",
			History:     1,
		})
		if err != nil {
			return err
		}
	}
	record.TaskID = strings.TrimSpace(record.TaskID)
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = kv.Put(taskKVKey(record.TaskID), payload)
	return err
}

func syntheticTaskKVRecord(taskID string, command string, state string) taskKVRecord {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	taskID = strings.TrimSpace(taskID)
	return taskKVRecord{
		TaskID:    taskID,
		Command:   strings.TrimSpace(command),
		Topic:     "task." + taskID,
		Host:      "kv-test-host",
		Mode:      "foreground",
		State:     strings.TrimSpace(state),
		CreatedAt: now,
		UpdatedAt: now,
	}
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

func waitForTaskListContains(rt *support.Runtime, state string, timeout time.Duration, patterns ...string) (string, error) {
	deadline := time.Now().Add(timeout)
	lastOut := ""
	args := []string{"repl", "src_v3", "task", "list", "--state", state, "--count", "20"}
	for time.Now().Before(deadline) {
		out, err := rt.RunDialtone(args...)
		if err == nil {
			lastOut = out
			matched := true
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
	return lastOut, fmt.Errorf("timed out waiting for task list state=%s to contain %s\n%s", state, strings.Join(patterns, ", "), lastOut)
}

func waitForTaskLogContains(rt *support.Runtime, taskID string, timeout time.Duration, patterns ...string) (string, error) {
	deadline := time.Now().Add(timeout)
	lastOut := ""
	for time.Now().Before(deadline) {
		out, err := rt.RunDialtone("repl", "src_v3", "task", "log", "--task-id", taskID, "--lines", "50")
		if err == nil {
			lastOut = out
			matched := true
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
	return lastOut, fmt.Errorf("timed out waiting for task log %s to contain %s\n%s", taskID, strings.Join(patterns, ", "), lastOut)
}

func allocateLocalNATSURL() string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "nats://127.0.0.1:46322"
	}
	defer ln.Close()
	return "nats://" + strings.TrimSpace(ln.Addr().String())
}

func runProcessClean(rt *support.Runtime) error {
	out, err := rt.RunDialtone("repl", "src_v3", "process-clean")
	if err != nil {
		return fmt.Errorf("process-clean failed: %w\n%s", err, out)
	}
	if !strings.Contains(out, "process-clean complete") {
		return fmt.Errorf("process-clean output missing completion marker\n%s", out)
	}
	return nil
}

func bestEffortKillManagedProcess(pid int) {
	if pid <= 0 {
		return
	}
	_ = proc.KillManagedProcess(pid)
}

func stopTask(rt *support.Runtime, taskID string, timeout time.Duration) error {
	out, err := rt.RunDialtone("repl", "src_v3", "task", "kill", "--task-id", taskID)
	if err != nil {
		return fmt.Errorf("task kill failed for %s: %w\n%s", taskID, err, out)
	}
	for _, expected := range []string{
		"Stopping task " + taskID,
		"Stop signal sent to task " + taskID + ".",
	} {
		if !strings.Contains(out, expected) {
			return fmt.Errorf("task kill output for %s missing %q\n%s", taskID, expected, out)
		}
	}
	_, err = waitForTaskShowContains(rt, taskID, timeout, "State: done")
	return err
}
