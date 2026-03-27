# REPL src_v3 Test Plan

## Purpose

This file proposes the test plan for the new `repl src_v3` design:

- every `./dialtone.sh ...` call becomes a queued task
- the CLI returns immediately with a `task-id`
- the leader keeps task and service state in NATS
- PID is late runtime state, not the public identity
- services like `chrome src_v3` are managed as desired and observed state

The goal is to test `repl src_v3` as a process manager and control plane for both local and remote processes.

## Current Coverage Snapshot

The existing suite already covers useful ground:

- leader autostart and leader reuse
- bootstrap workspace creation
- help surfaces
- config writes into `env/dialtone.json`
- SSH path smoke checks
- Cloudflare tunnel and tsnet smoke checks
- legacy service heartbeat visibility
- legacy background and foreground process behavior
- legacy `subtone-list`, `subtone-log`, and `subtone-attach`
- transcript-driven `dialtone>` checks

The main gap is that the current suite is still testing the legacy PID-first and `subtone`-first model.

It does not yet robustly test:

- task-id issuance
- queued task scheduling
- task state snapshots in NATS
- desired vs observed service state
- remote service reconciliation on hosts like `legion`
- remote PID assignment
- crash, panic, hung task, and restart behavior under the new design

## Testing Principles

The new REPL suite should follow these rules:

1. Prefer testing through `./dialtone.sh` and `dialtone>` the way a real user would use the system.
2. Treat direct Go helper calls as support tooling, not the primary proof.
3. Assert task-id-first behavior before asserting PID details.
4. Assert NATS state as the source of truth for tasks and services.
5. Test local and remote behavior with the same semantics.
6. Keep at least one simple daemon fixture that is much easier to reason about than `chrome src_v3`.
7. Keep panic, crash, hang, and forced-exit scenarios as first-class tests.
8. Test both user-facing modes:
   - short-lived CLI submission like `./dialtone.sh chrome src_v3 status --host legion --role dev`
   - long-lived interactive REPL usage from plain `./dialtone.sh` with slash commands
9. Assert that the CLI returns quickly with a `task-id` while the leader continues to emit later lifecycle messages through `dialtone>`.
10. Treat the top-level `dialtone>` stream as non-deterministic when multiple background or service-class tasks are active.
11. Assert transcript correctness per task id, not by one global total ordering of all lines.
12. Prove that one task can fail with a nonzero exit code and error lines without suppressing unrelated tasks that still succeed.

## New Test Fixtures We Need

## 1. Simple REPL Test Daemon

We should add a very small daemon fixture for REPL testing, for example:

- `src/plugins/repl/src_v3/testdaemon/src_v1`

This daemon should be usable:

- locally
- remotely on `legion`
- through the same REPL service/task model used by real plugins

Its job is to simulate the process shape of `chrome src_v3` without any browser complexity.

## 2. Test Daemon Requirements

The test daemon should:

- run as a long-lived process
- publish a heartbeat
- keep a tiny in-memory state map
- log lifecycle events
- publish logs through the shared logs library and NATS subjects
- expose one simple request/reply command surface over NATS
- report its host, PID, started time, and current mode
- optionally open a TCP port so we can test port reporting
- optionally spawn a child process so we can test cleanup behavior
- optionally panic, crash, hang, or exit with a chosen code

## 3. Standard Request Format

The daemon command contract should intentionally resemble `chrome src_v3`.

Suggested request:

```json
{
  "command": "status",
  "service_name": "repl-testd-dev",
  "role": "dev",
  "task_id": "task-20260327-abc123",
  "value": "hello",
  "expected": "ok",
  "timeout_ms": 5000
}
```

Suggested response:

```json
{
  "ok": true,
  "error": "",
  "host": "legion",
  "pid": 25516,
  "service_name": "repl-testd-dev",
  "state": "running",
  "started_at": "2026-03-27T16:00:00Z",
  "task_id": "task-20260327-abc123",
  "log_lines": ["testd: handled status"]
}
```

Suggested commands:

- `status`
- `ping`
- `echo`
- `sleep`
- `set-state`
- `get-state`
- `emit-log`
- `emit-progress`
- `open-port`
- `spawn-child`
- `set-health`
- `drop-heartbeat`
- `resume-heartbeat`
- `panic`
- `crash`
- `exit-code`
- `hang`
- `shutdown`

This daemon becomes the standard fixture for REPL process-manager tests, much like `chrome src_v3` is the real production fixture for browser service tests.

## 4. Crash Fixture Behavior

The same test daemon should support explicit bad behaviors:

- panic on startup
- panic on command
- clean nonzero exit
- hard crash
- hang forever
- stop heartbeating while still alive
- heartbeat while unhealthy

These are essential for testing the leader as a control plane, not just a happy-path launcher.

## Test Categories

## A. CLI Task Submission

These tests should confirm the new public behavior of `./dialtone.sh`.

Required tests:

- `shell-routed-command-returns-task-id-when-leader-missing`
- `shell-routed-command-returns-task-id-when-leader-already-running`
- `cli-returns-before-task-finishes`
- `dialtone-output-is-task-first-not-pid-first`
- `task-id-appears-before-pid-assignment`
- `task-log-path-is-returned-at-queue-time`
- `leader-later-emits-pid-log-and-exit-for-cli-submitted-task`
- `one-command-per-invocation-still-enforced`

Expected `dialtone>` checks:

- `Request received.`
- `Task queued as <task-id>.`
- `Task room: task.<task-id>`
- `Task log: ...task-<task-id>.log`

These tests should fail if the CLI still says `Spawning subtone` or leads with a PID.

Timing assertions:

- the command returns before the queued task reaches final state
- the task snapshot exists in NATS before PID assignment
- later lifecycle updates are still emitted by the leader

## B. Interactive REPL Session

These tests should prove that plain `./dialtone.sh` opens a long-lived REPL and that users can keep interacting with it through slash commands.

Required tests:

- `plain-dialtone-sh-starts-long-running-repl-session`
- `repl-session-shows-connected-and-ready-messages`
- `slash-command-queues-task-and-keeps-session-alive`
- `repl-session-shows-pid-log-and-exit-after-task-is-queued`
- `multiple-slash-commands-work-in-one-session`
- `repl-session-remains-usable-while-a-longer-task-is-running`
- `repl-session-can-show-remote-legion-task-lifecycle`
- `repl-session-keeps-index-output-high-level`

Expected `dialtone>` checks:

- `Connected to repl.room.index ...`
- `Leader online ...`
- `Shared REPL session ready ...`
- `Request received.`
- `Task queued as ...`
- later `assigned pid`, `log confirmed`, `running`, and `exited with code ...`

These tests should use a transcript style that looks like a real user session:

```text
dialtone> Connected to repl.room.index via nats://127.0.0.1:46222
dialtone> Leader online on DIALTONE-SERVER
dialtone> Shared REPL session ready in room index.
host-name> /chrome src_v3 status --host legion --role dev
host-name> /proc src_v1 sleep 20
host-name> /proc src_v1 emit after-sleep
```

## C. Task Queue And Scheduler

These tests should validate the new queue semantics.

Required tests:

- `one-foreground-task-runs-at-a-time`
- `second-foreground-task-stays-queued-until-first-finishes`
- `queued-task-transitions-queued-assigned-starting-running-succeeded`
- `cancel-queued-task-before-start`
- `cancel-running-task`
- `background-policy-does-not-break-foreground-slot`

Important assertions:

- queue order is visible in NATS state
- the second task gets no PID until scheduled
- the CLI still returns immediately for both tasks
- background-capable tasks can still produce interleaved lifecycle lines in the shared transcript

## D. Task State In NATS

These tests should prove the leader is keeping canonical state in NATS.

Required tests:

- `task-submit-writes-task-snapshot`
- `task-events-stream-matches-task-snapshot`
- `task-pid-appears-late-in-state`
- `task-exit-code-persists-after-finish`
- `task-log-key-persists-even-if-process-never-starts`
- `leader-restart-recovers-task-state-from-nats`

These should verify:

- task submit reply
- task event subject
- task snapshot key
- queue state key

## E. Task Logs

These tests should move logging away from PID-first assumptions.

Required tests:

- `task-log-name-is-based-on-task-id`
- `task-log-is-created-before-pid-assignment`
- `task-log-contains-pid-after-start`
- `task-log-survives-process-crash`
- `task-log-can-be-read-by-task-id`
- `legacy-subtone-log-command-can-find-task-log-during-migration`

This is where we prove the old log model has really changed.

## F. Local Service Reconciliation

These tests should treat services as desired and observed state.

Required tests:

- `service-start-creates-desired-state-and-reconcile-task`
- `service-status-reads-observed-state`
- `service-stop-clears-desired-running-state`
- `service-restart-creates-new-instance-id`
- `service-reuse-does-not-spawn-duplicate-process`
- `service-observed-state-tracks-pid-log-and-health`

Test fixture:

- local `repl-testd` daemon

## G. Remote Service Reconciliation

These are some of the most important new tests.

Required tests:

- `remote-service-start-on-legion-returns-task-id-immediately`
- `remote-service-observed-state-shows-legion-host`
- `remote-service-pid-is-remote-pid`
- `remote-service-command-roundtrip-via-testd`
- `remote-service-log-path-is-returned`
- `remote-service-stop-updates-observed-state`
- `remote-service-reconcile-restores-missing-daemon`

These tests should make the remote process model obvious:

- the task is local to the REPL control plane
- the PID may belong to `legion`
- service state in NATS is the thing we trust

## H. Simple Remote Command Protocol

These tests should use the simple test daemon command surface as the remote-process fixture.

Required tests:

- `remote-testd-status-command`
- `remote-testd-echo-command`
- `remote-testd-set-state-get-state`
- `remote-testd-sleep-command-respects-timeout`
- `remote-testd-open-port-reports-port-count`
- `remote-testd-spawn-child-and-cleanup`

These tests should mimic the style of `chrome src_v3` commands without requiring Chrome.

They should also prove log behavior through NATS, for example:

- `emit-log --level error --tag fail --message boom`
- `emit-progress --count 5 --delay-ms 250`
- `set-health degraded`
- `drop-heartbeat`

## I. Crash, Panic, And Hang Recovery

These tests are essential for hardening the leader.

Required tests:

- `service-panics-on-start-and-task-fails-cleanly`
- `service-panics-on-command-and-observed-state-goes-unhealthy`
- `service-hard-crash-triggers-reconcile`
- `hung-task-can-be-canceled`
- `hung-service-heartbeat-timeout-marks-observed-state-degraded`
- `leader-does-not-lose-task-log-on-panic`
- `leader-does-not-block-new-queue-submissions-while-reporting-crash`

These should use the test daemon, not Chrome.

## J. REPL Transcript Contract

We should keep transcript-driven tests as a first-class part of the suite.

Required tests:

- `dialtone-output-uses-task-phrasing`
- `dialtone-output-never-prints-raw-json`
- `dialtone-output-never-prints-stack-trace-by-default`
- `dialtone-output-shows-queued-then-running-then-final-state`
- `dialtone-output-shows-log-path-before-task-starts`
- `dialtone-output-shows-pid-assignment-after-queue-time`
- `dialtone-output-keeps-index-room-clean`
- `task-room-can-carry-detailed-lifecycle-events`
- `interactive-repl-transcript-shows-multiple-user-commands-in-one-session`
- `parallel-background-task-lines-can-interleave`
- `failed-background-task-lines-include-task-id-exit-code-and-error-text`
- `successful-and-failed-tasks-can-complete-in-one-shared-transcript`
- `transcript-assertions-allow-global-reordering-but-require-per-task-lifecycle`

This preserves the human UX while the backend changes.

Transcript assertion rules:

- do not snapshot one single total line order when multiple tasks are active
- group lifecycle lines by `task-id`
- require each task to show its own valid lifecycle
- allow unrelated lines from other tasks to appear between those lifecycle lines
- require failed tasks to show both a nonzero exit code and at least one user-facing error line
- require successful tasks to continue reporting completion even when another task fails nearby

## K. Config And Bootstrap

These tests should assert that config is coming from `env/dialtone.json`.

Required tests:

- `leader-writes-active-nats-config-into-env-dialtone-json`
- `task-submit-uses-env-dialtone-json-by-default`
- `service-reconcile-uses-env-dialtone-json-by-default`
- `bootstrap-workspace-copies-required-repl-fields`
- `task-tests-fail-cleanly-when-config-is-missing`

## L. Multi-Host And Isolation

These tests should validate that many services can be tracked at once.

Required tests:

- `service-state-is-isolated-by-host-and-service-name`
- `legion-and-local-can-run-same-service-name-with-different-instance-ids`
- `role-like-service-names-do-not-collide`
- `multiple-remote-services-heartbeat-without-state-bleed`
- `remote-task-for-legion-does-not-overwrite-local-task-state`

## M. Migration And Compatibility

We still need temporary tests during the transition.

Required tests:

- `legacy-subtone-list-shows-task-backed-runs`
- `legacy-subtone-log-can-read-task-backed-log`
- `legacy-service-list-reads-new-service-state`
- `legacy-pid-based-views-do-not-become-the-source-of-truth`

These tests should be deleted once the migration is complete.

## N. Task And Service Inspection Commands

These tests should prove that the operator can inspect running local and remote work directly instead of inferring everything from the shared transcript.

Required tests:

- `task-list-shows-running-remote-chrome-daemon-with-task-id-and-remote-pid`
- `task-list-shows-running-command-and-service-tasks-together`
- `task-show-includes-host-service-pid-room-log-and-last-error`
- `service-list-shows-remote-daemon-owner-task-and-health`
- `service-show-links-service-state-back-to-owning-task`
- `task-kill-stops-a-running-command-task`
- `task-kill-stops-a-running-remote-service-task`
- `task-kill-updates-final-exit-state-for-the-target-task`

Important assertions:

- `task list` includes task id, host, kind, state, and PID
- remote services like `chrome src_v3` show their remote PID, not a fake local PID
- `service list` shows the owning task id for the daemon
- killing one task does not remove unrelated running tasks from the list

## O. NATS Logs Integration

These tests should validate the target logging contract with the logs plugin.

Required tests:

- `task-log-command-reads-durable-task-log-by-task-id`
- `logs-stream-can-follow-task-subject`
- `logs-stream-can-follow-service-subject`
- `error-filter-subject-shows-only-failed-task-lines`
- `fail-tag-filter-shows-failed-testdaemon-lines`
- `parallel-tasks-publish-to-separate-task-subjects`
- `remote-service-publishes-to-service-subject`

Important assertions:

- task logs are durable and task-id-first
- live `logs src_v1 stream` output matches the same underlying NATS log lines
- service log subjects continue working while task status changes
- error filters remain useful when multiple tasks are active

## dialtone Transcript Scenarios We Should Explicitly Test

These should all be transcript-based and should go through `./dialtone.sh`.

### 1. Local one-shot task

```text
host-name> /proc src_v1 emit hello
dialtone> Request received.
dialtone> Task queued as task-...
dialtone> Task room: task.task-...
dialtone> Task log: ...task-....log
dialtone> Task task-... assigned pid ...
dialtone> Task task-... exited with code 0.
```

### 2. Non-blocking CLI plus later leader lifecycle

```text
host-name> /chrome src_v3 status --host legion --role dev
dialtone> Request received.
dialtone> Task queued as task-...
dialtone> Task room: task.task-...
dialtone> Task log: ...task-....log

# Later, from the leader-backed transcript stream
dialtone> Task task-... assigned pid 25516 on legion.
dialtone> chrome service on legion role=dev is healthy.
dialtone> Task task-... exited with code 0.
```

### 3. Interactive REPL with multiple slash commands

```text
dialtone> Connected to repl.room.index via nats://127.0.0.1:46222
dialtone> Leader online on DIALTONE-SERVER
dialtone> Shared REPL session ready in room index.

host-name> /chrome src_v3 status --host legion --role dev
dialtone> Request received.
dialtone> Task queued as task-a...
dialtone> Task room: task.task-a...
dialtone> Task log: ...task-a....log
dialtone> Task task-a... assigned pid 25516 on legion.
dialtone> chrome service on legion role=dev is healthy.
dialtone> Task task-a... exited with code 0.

host-name> /proc src_v1 sleep 20
dialtone> Request received.
dialtone> Task queued as task-b...
dialtone> Task room: task.task-b...
dialtone> Task log: ...task-b....log

host-name> /proc src_v1 emit after-sleep
dialtone> Request received.
dialtone> Task queued as task-c...
dialtone> Task task-c... is queued behind 1 foreground task.
```

### 4. Remote service start on `legion`

```text
host-name> /repl-testd src_v1 service --host legion --mode start --name remote-dev
dialtone> Request received.
dialtone> Task queued as task-...
dialtone> Service reconcile queued for repl-testd remote-dev on legion.
dialtone> Task log: ...task-....log
```

Later status:

```text
host-name> /repl-testd src_v1 status --host legion --name remote-dev
dialtone> Request received.
dialtone> Task queued as task-...
dialtone> Service repl-testd remote-dev on legion is running.
dialtone> Remote pid: 25516
```

### 5. Remote service crash and recovery

```text
host-name> /repl-testd src_v1 command --host legion --name remote-dev --command panic
dialtone> Request received.
dialtone> Task queued as task-...
dialtone> Service repl-testd remote-dev on legion became unhealthy.
dialtone> Recovery task queued as task-...
```

### 6. Queued foreground contention

```text
host-name> /proc src_v1 sleep 20
dialtone> Request received.
dialtone> Task queued as task-a...

host-name> /proc src_v1 emit after-sleep
dialtone> Request received.
dialtone> Task queued as task-b...
dialtone> Task task-b... is queued behind 1 foreground task.
```

### 7. Interleaved background tasks with mixed exit codes

```text
dialtone> Connected to repl.room.index via nats://127.0.0.1:46222
dialtone> Leader online on DIALTONE-SERVER
dialtone> Shared REPL session ready in room index.

host-name> /proc src_v1 sleep 20
dialtone> Request received.
dialtone> Task queued as task-sleep...

host-name> /ssh src_v1 run --host grey --cmd 'echo ready'
dialtone> Request received.
dialtone> Task queued as task-ok...

host-name> /ssh src_v1 run --host grey --cmd 'echo boom >&2; exit 17'
dialtone> Request received.
dialtone> Task queued as task-fail...

dialtone> Task task-ok... assigned pid 51102 on grey.
dialtone> Task task-fail... assigned pid 51108 on grey.
dialtone> Task task-sleep... assigned pid 41122.
dialtone> Task task-ok... exited with code 0.
dialtone> ERROR task task-fail... on grey exited with code 17.
dialtone> ERROR task task-fail... stderr: boom
dialtone> Task task-sleep... exited with code 0.
```

Required assertions:

- the exact ordering between `task-ok`, `task-fail`, and `task-sleep` is allowed to vary
- each task still has a coherent local lifecycle
- the failed task carries its own error context
- the successful tasks still report completion

### 8. Listing a running remote Chrome daemon task

```text
host-name> /chrome src_v3 service --host legion --mode start --role dev
dialtone> Request received.
dialtone> Task queued as task-20260327-chr001.
dialtone> Task task-20260327-chr001 assigned pid 25516 on legion.
dialtone> chrome service on legion role=dev is healthy.
dialtone> Task task-20260327-chr001 exited with code 0.

host-name> /repl src_v3 task list --state running --host legion
dialtone> Running tasks:
dialtone> TASK ID                  KIND      STATE    HOST    SERVICE/COMMAND           PID    EXIT
dialtone> task-20260327-chr001     service   running  legion  chrome-src-v3-dev        25516  -
```

Required assertions:

- `task list` shows the owning task id for the remote daemon
- the PID shown is the remote daemon PID on `legion`
- the service row is still present after the start task itself has completed

### 9. Reading task and service logs through REPL and logs plugin

```text
host-name> /repl src_v3 task log --task-id task-20260327-chr001 --lines 4
dialtone> Streaming task log for task-20260327-chr001
[T+0000s|INFO|plugins/chrome/src_v3/ops.go:412] deploy requested for legion role=dev
[T+0001s|INFO|plugins/chrome/src_v3/daemon.go:221] daemon connected to repl manager

host-name> /logs src_v1 stream --topic 'logs.service.legion.chrome-src-v3-dev'
dialtone> Attached log stream logs.service.legion.chrome-src-v3-dev
[T+0002s|INFO|plugins/chrome/src_v3/browser.go:301] chrome started headed pid=25516
[T+0003s|ERROR|plugins/chrome/src_v3/actions.go:211] wait-aria timeout label=Open Camera
```

Required assertions:

- `task log` and `logs stream` surface compatible information
- live service logs can continue after the task log command exits
- error lines are visible through both the task log and error-filter topics

### 10. Killing a remote service-owned task

```text
host-name> /repl src_v3 task kill --task-id task-20260327-chr001
dialtone> Request received.
dialtone> Task queued as task-20260327-kill01.
dialtone> Kill requested for task-20260327-chr001.
dialtone> Target task task-20260327-chr001 is running on legion pid 25516.
dialtone> Service chrome-src-v3-dev on legion moved to stopping.
dialtone> Target task task-20260327-chr001 exited with code 143.
dialtone> Task task-20260327-kill01 exited with code 0.
```

Required assertions:

- the kill request itself gets its own task id
- the target task transitions to a stopped final state
- later `task list` or `service list` no longer report the daemon as running

### 11. Test-daemon emitted logs and simulated errors

```text
host-name> /repl-testd src_v1 command --host legion --name remote-dev --command emit-log --level error --tag fail --message boom
dialtone> Request received.
dialtone> Task queued as task-emit...
dialtone> ERROR task task-emit... service repl-testd remote-dev emitted error log: boom
dialtone> Task task-emit... exited with code 0.

host-name> /logs src_v1 stream --topic 'logfilter.tag.fail.>'
dialtone> Attached log stream logfilter.tag.fail.>
[T+0000s|ERROR|plugins/repl/src_v3/testdaemon/src_v1/main.go:141] [FAIL] boom
```

Required assertions:

- the test daemon can publish error and tagged log lines on demand
- `logs src_v1` filters can isolate those lines
- the REPL transcript still stays short while the detailed log stream remains available

## Proposed New Test Suites

Suggested new suite layout:

- `11_task_submission`
- `12_interactive_repl_session`
- `13_task_queue`
- `14_task_state_nats`
- `15_task_logs`
- `16_local_service_reconcile`
- `17_remote_service_reconcile`
- `18_test_daemon_protocol`
- `19_crash_and_recovery`
- `20_task_transcript_contract`
- `21_multi_host_state`
- `22_migration_compat`

The current `00_process_manager` and `10_repl_logging_contract` suites should be split and renamed as we move from `subtone` semantics to task semantics.

## Implementation Order

Recommended build order:

1. Add the simple local test daemon.
2. Add task-id-first CLI transcript tests.
3. Add long-lived interactive REPL slash-command transcript tests.
4. Add NATS task snapshot and task log tests.
5. Add local service desired/observed state tests.
6. Add remote `legion` test-daemon deployment and command tests.
7. Add crash, panic, and hang tests.
8. Add migration compatibility tests.
9. Update the old `subtone`-named suites to either migrate or retire.

## Success Criteria

We should consider the new REPL design well-tested when:

- every normal `./dialtone.sh` request is proven to return a `task-id`
- plain `./dialtone.sh` is proven to open a long-lived REPL session that accepts multiple slash commands
- every task is visible in NATS state before a PID exists
- local and remote PIDs are clearly treated as observed runtime metadata
- long-lived services can be reconciled, reused, stopped, and recovered
- crash and hang cases do not wedge the queue
- the operator can understand progress from `dialtone>` in both CLI mode and interactive REPL mode without needing raw internals
- the test daemon covers enough of the control-plane contract that Chrome is no longer the only realistic remote-process fixture
