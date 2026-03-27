# REPL src_v3 Test Plan

## Purpose

This plan defines the test strategy for the task-first `repl src_v3` system described in:

- [README.md](/C:/Users/timca/dialtone/README.md)
- [README.md](/C:/Users/timca/dialtone/src/plugins/repl/src_v3/README.md)
- [README.md](/C:/Users/timca/dialtone/src/plugins/chrome/src_v3/README.md)
- [DESIGN.md](/C:/Users/timca/dialtone/src/plugins/repl/src_v3/DESIGN.md)

The runtime contract is:

- every `./dialtone.sh ...` request becomes one queued task
- every task gets a `task-id` immediately
- every task gets a task room and task log immediately
- PID is later runtime state, not the public identity
- NATS is the source of truth for task and service state
- `dialtone>` stays short, high-level, and task-oriented
- `chrome src_v3` is managed through the REPL service model, not through ad hoc launcher behavior

The goal of this plan is to prove the task-id-first system directly.

## Public Contract From The Docs

The tests should be written against the public contract, not current implementation accidents.

Required top-level task transcript:

```text
host-name> /chrome src_v3 status --host legion --role dev
dialtone> Request received.
dialtone> Task queued as task-20260327-abc123.
dialtone> Task room: task.task-20260327-abc123
dialtone> Task log: ~/.dialtone/logs/task-20260327-abc123.log
dialtone> Task task-20260327-abc123 assigned pid 25516 on legion.
dialtone> chrome service on legion role=dev is healthy.
dialtone> Task task-20260327-abc123 exited with code 0.
```

Required operator surface:

```bash
./dialtone.sh repl src_v3 task list
./dialtone.sh repl src_v3 task show --task-id <task-id>
./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 200
./dialtone.sh repl src_v3 task kill --task-id <task-id>
./dialtone.sh repl src_v3 service list --host legion
./dialtone.sh repl src_v3 service show --host legion --name chrome-src-v3-dev
./dialtone.sh logs src_v1 stream --topic 'logs.task.<task-id>'
```

Required failures:

- top-level output must not lead with PID
- top-level output must not say `Spawning subtone`
- top-level output must point users to task and service inspection commands
- normal task submission must not depend on trailing `&`

## Non-Goals

The public contract does not include:

- PID as the primary task identity
- process-oriented operator commands as the primary inspection path
- PID-named log files as the public log model
- trailing `&` as the normal backgrounding contract
- transcript wording centered on process launch rather than task queueing

## Start Here: 8 Steps

These are the first 8 steps to get the code moving toward the target design.

1. Create a real task object in NATS before process launch and return `task-id`, task room, and task log immediately.
2. Replace top-level transcript wording so every command starts with `Request received.` and `Task queued as ...`, never `Spawning subtone`.
3. Introduce task-backed log creation and task-backed task state so log lookup is no longer derived from PID.
4. Add `repl src_v3 task list`, `task show`, `task log`, and `task kill` with NATS task state as the primary source of truth.
5. Add a simple REPL test daemon for queue, progress, crash, hang, and recovery tests before depending on Chrome behavior.
6. Prove local task queue semantics and interleaved multi-task transcripts with the daemon and simple shell commands.
7. Prove real remote SSH/deploy behavior on `grey`, then prove service reconciliation and reuse on `legion`.
8. Move `chrome src_v3` fully onto desired/observed REPL service state and remove process-oriented public surfaces from the runtime and docs.

## WSL tmux Test Workflow

Use the Windows wrapper so every WSL run stays visible in the persistent tmux pane:

```powershell
.\wsl-tmux.cmd clean-state
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 format"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 test --filter interactive-command-index-lifecycle-contract"
.\wsl-tmux.cmd read
```

When the code changes affect the bootstrap tarball path, restart REPL/bootstrap processes first so isolated test repos pull a fresh snapshot:

```powershell
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 process-clean"
```

## Testing Principles

1. Test through `./dialtone.sh` and `dialtone>` first.
2. Assert task identity before asserting PID details.
3. Assert NATS state as the main source of truth.
4. Test the CLI and interactive REPL as two views of the same task system.
5. Prefer task-oriented assertions over implementation-oriented assertions.
6. Treat interleaving as normal when multiple tasks are active.
7. Validate remote and local flows with the same task lifecycle semantics.
8. Make crash, panic, hang, timeout, and restart behavior first-class tests.
9. Keep Chrome service tests focused on service reconciliation, not generic task queue behavior.
10. Fail any test that depends on process-first naming or transcript structure.

## Execution Tiers

### Tier 0. Bootstrap And Install

- validates tmp bootstrap, config creation, installer behavior, and managed toolchain setup
- proves `repl src_v3 install`, workspace bootstrap, and environment resolution

### Tier 1. Fast Local Task Runtime

- validates task submission, queue semantics, task state, task logs, task operator commands, and REPL transcript shape
- should be the default inner loop during migration

### Tier 2. Stable Remote SSH And Deploy

- validates real SSH probe, run, copy, and deploy behavior on `grey`
- proves that remote work still follows the same task-first lifecycle

### Tier 3. Remote Service Reconciliation

- validates long-lived service behavior on `legion`
- proves `chrome src_v3` service start, reuse, recovery, stop, and task ownership

### Tier 4. Same-Host WSL Diagnostics

- covers same-host WSL mirror-mode behavior when useful
- is a diagnostic lane for this workstation, not the core design gate

## Test Fixtures We Need

## 1. Simple REPL Test Daemon

Add a small fixture plugin:

- `src/plugins/repl/src_v3/testdaemon/src_v1`

It should:

- run locally or remotely
- expose a request/reply command surface over NATS
- emit logs through the shared logs library
- publish heartbeats
- report host, PID, ports, and started time
- support explicit `sleep`, `panic`, `crash`, `exit-code`, `hang`, `shutdown`, and `emit-progress`

This fixture should carry most control-plane tests so Chrome tests can stay focused on browser service behavior.

## 2. Remote Targets

- `grey`: canonical SSH and deploy target
- `legion`: canonical long-lived service target
- same-host `wsl`: diagnostic-only lane when needed

## Test Categories

## A. CLI Task Submission

Required tests:

- `shell-routed-command-returns-task-id-when-leader-missing`
- `shell-routed-command-returns-task-id-when-leader-running`
- `cli-returns-before-task-finishes`
- `task-id-appears-before-pid-assignment`
- `task-log-path-is-known-at-queue-time`
- `task-room-is-known-at-queue-time`
- `dialtone-output-is-task-first`
- `one-command-per-invocation-still-enforced`

Required assertions:

- `Request received.`
- `Task queued as task-...`
- `Task room: task.task-...`
- `Task log: ...task-...`

These tests should fail if the transcript says `Spawning subtone` or starts with a PID.

## B. Interactive REPL Session

Required tests:

- `plain-dialtone-opens-shared-repl-session`
- `slash-command-queues-task-and-keeps-session-open`
- `multiple-slash-commands-can-run-in-sequence`
- `repl-shows-task-id-for-every-command`
- `repl-keeps-accepting-input-while-earlier-task-runs`

Required assertions:

- `dialtone> Connected to repl.room.index ...`
- `dialtone> Leader online ...`
- `dialtone> Shared REPL session ready in room index.`

## C. Task Queue And Scheduler

Required tests:

- `queued-task-exists-before-launch`
- `second-task-stays-queued-while-first-is-running`
- `scheduler-starts-next-task-after-first-exits`
- `task-state-transitions-queued-running-exited`
- `parallel-service-and-command-tasks-can-interleave-cleanly`

## D. Task State In NATS

Required tests:

- `task-submit-creates-nats-record`
- `task-record-includes-task-id-room-log-host-command`
- `task-record-updates-pid-after-launch`
- `task-record-updates-exit-code-on-finish`
- `task-list-reads-from-nats-state`
- `task-show-reads-from-nats-state`

## E. Task Logs

Required tests:

- `task-log-file-created-at-queue-time`
- `task-log-appends-lifecycle-and-plugin-output`
- `task-log-command-reads-by-task-id`
- `logs-stream-task-subject-matches-task-log`
- `error-lines-appear-in-task-log-and-top-level-summary`

## F. Task Operator Commands

Required tests:

- `task-list-shows-running-and-finished-tasks`
- `task-show-displays-room-log-host-pid-exit`
- `task-log-prints-recent-lines-by-task-id`
- `task-kill-stops-running-task`
- `task-kill-updates-state-to-stopping-or-exited`

## G. Local Service Reconciliation

Required tests:

- `service-start-creates-desired-running-state`
- `status-reuses-existing-healthy-service`
- `service-stop-clears-desired-running-state`
- `service-list-shows-owner-task-and-health`
- `service-show-displays-desired-vs-observed-state`

## H. Remote SSH And Deploy On Grey

Required tests:

- `remote-grey-probe-queues-task-and-succeeds`
- `remote-grey-run-queues-task-and-returns-remote-pid`
- `remote-grey-copy-or-deploy-creates-task-log`
- `remote-grey-failure-propagates-error-lines-and-exit`

## I. Remote Chrome Service On Legion

Required tests:

- `chrome-service-start-queues-task-and-creates-service-state`
- `chrome-status-reuses-running-service`
- `chrome-command-reuses-service-pid`
- `chrome-service-recovery-replaces-missing-process`
- `chrome-service-stop-stops-owned-process`
- `service-list-on-legion-shows-owner-task`

## J. Failure, Timeout, And Recovery

Required tests:

- `task-panic-appears-as-error-and-nonzero-exit`
- `task-crash-appears-as-error-and-nonzero-exit`
- `task-timeout-produces-clear-task-error`
- `hung-task-can-be-killed-by-task-id`
- `missing-service-is-reconciled-on-next-command`
- `heartbeat-loss-marks-service-unhealthy`

## K. Interleaving And Isolation

Required tests:

- `multiple-running-tasks-produce-interleaved-but-coherent-output`
- `one-failing-task-does-not-hide-other-successful-tasks`
- `task-log-remains-isolated-under-interleaving`
- `service-output-does-not-corrupt-unrelated-task-state`

## L. Config And Bootstrap

Required tests:

- `bootstrap-creates-valid-env-dialtone-json`
- `add-host-updates-mesh-config`
- `task-runtime-resolves-repo-roots-correctly`
- `task-runtime-uses-managed-go-and-bun`

## M. Logs And Observability

Required tests:

- `logs-task-subject-exists-for-every-task`
- `logs-service-subject-exists-for-running-service`
- `logfilter-level-error-captures-task-failures`
- `task-log-and-logs-stream-surface-the-same-core-events`

## N. Multi-Host And Isolation

Required tests:

- `tasks-on-grey-and-legion-keep-distinct-host-state`
- `same-command-on-two-hosts-gets-two-task-ids`
- `service-owner-task-is-host-specific`
- `host-filtered-task-list-returns-only-target-host`

## Explicit dialtone> Transcript Scenarios

## 1. Local One-Shot Command

```text
host-name> /repl src_v3 help
dialtone> Request received.
dialtone> Task queued as task-20260327-help001.
dialtone> Task room: task.task-20260327-help001
dialtone> Task log: ~/.dialtone/logs/task-20260327-help001.log
dialtone> Task task-20260327-help001 assigned pid 41122.
dialtone> Task task-20260327-help001 exited with code 0.
```

## 2. Remote SSH Command

```text
host-name> /ssh src_v1 run --host grey --cmd hostname
dialtone> Request received.
dialtone> Task queued as task-20260327-ssh001.
dialtone> Task room: task.task-20260327-ssh001
dialtone> Task log: ~/.dialtone/logs/task-20260327-ssh001.log
dialtone> Task task-20260327-ssh001 assigned pid 51102 on grey.
dialtone> ssh run on grey: grey
dialtone> Task task-20260327-ssh001 exited with code 0.
```

## 3. Remote Chrome Service

```text
host-name> /chrome src_v3 service --host legion --mode start --role dev
dialtone> Request received.
dialtone> Task queued as task-20260327-chr001.
dialtone> Task room: task.task-20260327-chr001
dialtone> Task log: ~/.dialtone/logs/task-20260327-chr001.log
dialtone> Task task-20260327-chr001 assigned pid 25516 on legion.
dialtone> chrome service on legion role=dev is healthy.
dialtone> Task task-20260327-chr001 exited with code 0.
```

## 4. Failure

```text
host-name> /chrome src_v3 wait-aria --host legion --role dev --label "Open Camera" --timeout-ms 1500
dialtone> Request received.
dialtone> Task queued as task-20260327-wait001.
dialtone> Task room: task.task-20260327-wait001
dialtone> Task log: ~/.dialtone/logs/task-20260327-wait001.log
dialtone> Task task-20260327-wait001 assigned pid 25516 on legion.
dialtone> ERROR task task-20260327-wait001 on legion exited with code 28.
dialtone> ERROR task task-20260327-wait001 wait-aria timeout label=Open Camera
```

## 5. Interleaving

```text
host-name> /proc src_v1 sleep 20
dialtone> Task queued as task-20260327-sleep01.

host-name> /ssh src_v1 run --host grey --cmd 'echo ready'
dialtone> Task queued as task-20260327-echo01.

host-name> /ssh src_v1 run --host grey --cmd 'echo boom >&2; exit 17'
dialtone> Task queued as task-20260327-fail01.

dialtone> Task task-20260327-echo01 assigned pid 51102 on grey.
dialtone> Task task-20260327-fail01 assigned pid 51108 on grey.
dialtone> Task task-20260327-sleep01 assigned pid 41122.
dialtone> Task task-20260327-echo01 exited with code 0.
dialtone> ERROR task task-20260327-fail01 on grey exited with code 17.
dialtone> Task task-20260327-sleep01 exited with code 0.
```

## Proposed Suite Layout

- `00_bootstrap_install`
- `01_cli_task_submission`
- `02_interactive_repl`
- `03_task_state_and_logs`
- `04_task_operator_commands`
- `05_local_service_reconcile`
- `06_remote_ssh_grey`
- `07_remote_service_legion`
- `08_failure_recovery`
- `09_interleaving_and_isolation`
- `10_logs_and_observability`
- `11_same_host_wsl_diagnostics`

## End-To-End Commands To Use While Migrating

These are the commands to keep using during the migration loop:

```bash
./dialtone.sh repl src_v3 test --filter shell-routed-command-returns-task-id-when-leader-missing
./dialtone.sh repl src_v3 test --filter plain-dialtone-opens-shared-repl-session
./dialtone.sh repl src_v3 test --filter task-list-shows-running-and-finished-tasks
./dialtone.sh repl src_v3 test --filter remote-grey-run-queues-task-and-returns-remote-pid
./dialtone.sh repl src_v3 test --filter chrome-service-start-queues-task-and-creates-service-state
./dialtone.sh repl src_v3 test --filter multiple-running-tasks-produce-interleaved-but-coherent-output
```

The narrowest useful first command is:

```bash
./dialtone.sh repl src_v3 test --filter shell-routed-command-returns-task-id-when-leader-missing
```

The best local control-plane loop after that is:

```bash
./dialtone.sh repl src_v3 test --filter shell-routed-command-returns-task-id-when-leader-missing,plain-dialtone-opens-shared-repl-session,task-list-shows-running-and-finished-tasks
```

## Success Criteria

The migration is successful when all of these are true:

- every request returns a `task-id`
- every request gets a task room and task log before PID assignment
- the top-level transcript is task-first everywhere
- the CLI depends only on task-first wording
- `task list`, `task show`, `task log`, and `task kill` are the standard operator tools
- local and remote work share the same task lifecycle semantics
- `chrome src_v3` is fully managed as REPL service state on `legion`
- the task log and NATS subjects provide durable, inspectable state for every task
