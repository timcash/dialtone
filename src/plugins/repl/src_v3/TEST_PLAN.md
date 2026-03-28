# REPL src_v3 Test Plan

## Purpose

This plan defines the test strategy for the task-first `repl src_v3` system described in:

- [README.md](/C:/Users/timca/dialtone/README.md)
- [README.md](/C:/Users/timca/dialtone/src/plugins/repl/src_v3/README.md)
- [README.md](/C:/Users/timca/dialtone/src/plugins/chrome/src_v3/README.md)
- [DESIGN.md](/C:/Users/timca/dialtone/src/plugins/repl/src_v3/DESIGN.md)

The runtime contract is:

- queued task submission is the default for `./dialtone.sh ...`
- queued tasks get a `task-id`, task topic, and task log immediately
- user-facing output uses lowercase `dialtone>`
- PID is later runtime state, not the public identity
- NATS KV is the source of truth for durable task and service state
- the launch folder's `env/dialtone.json` is the default runtime config source, and `--env` can target another env root or file
- queued one-shot CLI commands print the queued task summary and a helpful `task log --task-id ... --lines N` command, then return immediately
- explicit query/operator commands stay foreground and print the requested data directly
- there is no public `subtone` language; `task` and `service` are the public domain terms
- `dialtone>` stays short, high-level, and task-oriented
- `chrome src_v3` is managed through the REPL service model, not through ad hoc launcher behavior
- service tasks publish heartbeats and the leader reconciles/restarts them if heartbeats stop

The goal of this plan is to prove the task-id-first system directly.
Generic service-control-plane tests and service transcript examples must use the `testdaemon` fixture so they do not depend on Chrome or any other plugin implementation.

## Public Contract From The Docs

The tests should be written against the public contract, not current implementation accidents.

Required one-shot CLI transcript:

```text
host-name> /testdaemon src_v1 service --host legion --mode status --name demo
dialtone> Request received.
dialtone> Task queued as task-20260327-abc123.
dialtone> Task topic: task.task-20260327-abc123
dialtone> Task log: ~/.dialtone/logs/task-20260327-abc123.log
dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-20260327-abc123 --lines 10
```

Required later REPL/task-log lifecycle for that same task:

```text
dialtone> Task task-20260327-abc123 assigned pid 25516 on legion.
dialtone> testdaemon service demo on legion is healthy.
dialtone> Task task-20260327-abc123 exited with code 0.
```

Required operator surface:

```bash
./dialtone.sh repl src_v3 task list
./dialtone.sh repl src_v3 task show --task-id <task-id>
./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 200
./dialtone.sh repl src_v3 task kill --task-id <task-id>
./dialtone.sh repl src_v3 service list --host legion
./dialtone.sh repl src_v3 service show --host legion --name demo
./dialtone.sh logs src_v1 stream --topic 'logs.task.<task-id>'
```

Required foreground query examples:

```text
./dialtone.sh proc src_v1 ps
No active managed processes.

./dialtone.sh repl src_v3 task log --task-id task-20260327-abc123 --lines 10
Task log: ~/.dialtone/logs/task-20260327-abc123.log
...
```

Required failures:

- top-level output must not lead with PID
- top-level output must not say `Spawning subtone`
- top-level output must not expose public `subtone-*` commands or names
- top-level output must not render uppercase `DIALTONE>` on the active `dialtone.sh` / `repl src_v3` path
- top-level output must point users to task and service inspection commands
- normal task submission must not depend on trailing `&`
- one-shot CLI submission must not block waiting for `assigned pid` or final exit lines
- foreground query/operator commands must not emit the queued-task transcript unless they actually create a task

## Non-Goals

The public contract does not include:

- PID as the primary task identity
- process-oriented operator commands as the primary inspection path
- PID-named log files as the public log model
- trailing `&` as the normal backgrounding contract
- transcript wording centered on process launch rather than task queueing

## Start Here: 8 Steps

These are the first 8 steps to get the code moving toward the target design.

1. Create a real task object in NATS KV before process launch and return `task-id`, task topic, task log, and log-inspection hint immediately.
2. Replace top-level transcript wording so every command starts with `Request received.` and `Task queued as ...`, never `Spawning subtone`.
3. Remove remaining public `subtone` names and process-first operator wording in favor of `task` and `service`.
4. Introduce task-backed log creation and task-backed task state so log lookup is no longer derived from PID.
5. Add `repl src_v3 task list`, `task show`, `task log`, and `task kill` with NATS KV task state as the primary source of truth.
6. Add heartbeat-driven service reconciliation so missed service heartbeats mark a service unhealthy and trigger restart/recovery.
7. Add a simple REPL test daemon for queue, progress, crash, hang, and recovery tests before depending on Chrome behavior.
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

## Current Verified Baseline

As of `2026-03-27`, these commands were rerun successfully against the WSL repo:

- `./dialtone.sh repl src_v3 format`
- `./dialtone.sh repl src_v3 build`
- `./dialtone.sh repl src_v3 test --filter shell-routed-command-autostarts-leader-when-missing`
- `./dialtone.sh repl src_v3 test --filter shell-routed-command-reuses-running-leader`

Visible `wsl-tmux.cmd` verification also confirmed the interactive startup transcript now uses the topic-first wording:

- `dialtone> Connected to repl.topic.index via ...`
- `dialtone> legion joined topic index (version=dev).`
- `dialtone> Leader online on DIALTONE-SERVER (topic=repl.topic.index nats=...)`
- `dialtone> Shared REPL session ready on topic index.`

Visible operator checks also confirmed the current task-inspection surface:

- `./dialtone.sh repl src_v3 task show --task-id <task-id>` now prints `Topic: index` instead of `Room: index`
- after `./dialtone.sh repl src_v3 process-clean`, a fresh routed task log shows `worker_log=/home/user/.dialtone/logs/task-worker-...` instead of `subtone-...`

One live shell-routed verification command also matched the queue-only contract:

```bash
./dialtone.sh proc src_v1 emit shell-contract-check
```

Expected transcript shape from that command:

```text
host-name> /proc src_v1 emit shell-contract-check
dialtone> Request received.
dialtone> Task queued as task-20260327-abc123.
dialtone> Task topic: task.task-20260327-abc123
dialtone> Task log: ~/.dialtone/logs/task-20260327-abc123.log
dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-20260327-abc123 --lines 10
```

If the leader is missing, an autostart preamble may appear before the routed command. That preamble should also migrate to `topic index` wording rather than legacy `room index`.

## Testing Principles

1. Test through `./dialtone.sh` and `dialtone>` first.
2. Assert task identity before asserting PID details.
3. Assert NATS KV state as the main source of truth for durable task/service state.
4. Test the CLI and interactive REPL as two views of the same task system.
5. Prefer task-oriented assertions over implementation-oriented assertions.
6. Treat interleaving as normal when multiple tasks are active.
7. Validate remote and local flows with the same task lifecycle semantics.
8. Make crash, panic, hang, timeout, and restart behavior first-class tests.
9. Keep Chrome tests focused on browser-specific behavior over the shared service layer, not generic service reconciliation.
10. Fail any test that depends on process-first naming or transcript structure.
11. Fail any test that exposes public `subtone` domain language instead of `task`/`service`.

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

- validates long-lived generic service behavior on `legion` with the `testdaemon` fixture
- proves service start, reuse, heartbeat, recovery, stop, and task ownership without depending on other plugin code

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
- expose a simple service command surface used by reconciliation tests
- emit logs through the shared logs library
- publish heartbeats
- report host, PID, ports, and started time
- support explicit `sleep`, `panic`, `crash`, `exit-code`, `hang`, `shutdown`, and `emit-progress`

This fixture should carry the generic service-control-plane suite, the service transcript examples in this plan, and the local/remote reconciliation tests. Chrome tests should stay focused on browser-specific behavior layered on top of the shared service model.

## 2. Remote Targets

- `grey`: canonical SSH and deploy target
- `legion`: canonical long-lived service target
- same-host `wsl`: diagnostic-only lane when needed

## Test Categories

## A. CLI Task Submission

Required tests:

- `shell-routed-command-autostarts-leader-when-missing`
- `shell-routed-command-reuses-running-leader`
- `shell-foreground-query-autostarts-leader-and-prints-direct-output`
- `cli-returns-before-task-finishes`
- `cli-prints-task-log-follow-up-command`
- `cli-uses-lowercase-dialtone-prefix`
- `task-id-appears-before-pid-assignment`
- `task-log-path-is-known-at-queue-time`
- `task-topic-is-known-at-queue-time`
- `dialtone-output-is-task-first`
- `one-command-per-invocation-still-enforced`

Required assertions:

- `Request received.`
- `Task queued as task-...`
- `Task topic: task.task-...`
- `Task log: ...task-...`
- `To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-... --lines 10`

These tests should fail if the transcript says `Spawning subtone`, starts with a PID, or waits for later lifecycle lines before returning.

Foreground query assertions:

- `./dialtone.sh proc src_v1 ps` prints direct process state instead of `Request received.` / `Task queued as ...`
- `./dialtone.sh repl src_v3 task log --task-id ... --lines N` prints the requested log lines directly
- foreground query commands may start the background leader first, but they stay synchronous and data-first

## B. Interactive REPL Session

Required tests:

- `plain-dialtone-opens-shared-repl-session`
- `slash-command-queues-task-and-keeps-session-open`
- `multiple-slash-commands-can-run-in-sequence`
- `interactive-repl-uses-lowercase-dialtone-prefix`
- `repl-shows-task-id-for-every-command`
- `repl-keeps-accepting-input-while-earlier-task-runs`

Required assertions:

- `dialtone> Connected to repl.topic.index ...`
- `dialtone> Leader online ...`
- `dialtone> Shared REPL session ready on topic index.`

## C. Task Queue And Scheduler

Required tests:

- `queued-task-exists-before-launch`
- `second-task-stays-queued-while-first-is-running`
- `scheduler-starts-next-task-after-first-exits`
- `task-state-transitions-queued-running-exited`
- `parallel-service-and-command-tasks-can-interleave-cleanly`

## D. Task State In NATS KV

Required tests:

- `task-submit-creates-nats-kv-record`
- `task-record-includes-task-id-topic-log-host-command`
- `task-record-updates-pid-after-launch`
- `task-record-updates-exit-code-on-finish`
- `task-list-reads-from-nats-kv-state`
- `task-show-reads-from-nats-kv-state`

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
- `task-show-displays-topic-log-host-pid-exit`
- `task-log-prints-recent-lines-by-task-id`
- `task-kill-stops-running-task`
- `task-kill-updates-state-to-stopping-or-exited`

## G. Local Service Reconciliation With testdaemon

These tests must run against `testdaemon` only. They should not rely on Chrome or any other plugin implementation to prove generic service behavior.

Required tests:

- `testdaemon-service-start-creates-desired-running-state`
- `testdaemon-status-reuses-existing-healthy-service`
- `testdaemon-service-stop-clears-desired-running-state`
- `testdaemon-service-list-shows-owner-task-and-health`
- `testdaemon-service-show-displays-desired-vs-observed-state`
- `testdaemon-service-heartbeat-updates-observed-state`
- `testdaemon-missed-service-heartbeat-marks-unhealthy`
- `leader-restarts-testdaemon-service-after-heartbeat-loss`

## H. Remote SSH And Deploy On Grey

Required tests:

- `remote-grey-probe-queues-task-and-succeeds`
- `remote-grey-run-queues-task-and-returns-remote-pid`
- `remote-grey-copy-or-deploy-creates-task-log`
- `remote-grey-failure-propagates-error-lines-and-exit`

## I. Remote testdaemon Service On Legion

These tests prove the remote service layer itself. They must use `testdaemon`, not Chrome, so the reconciliation contract is validated without browser-specific code.

Required tests:

- `remote-testdaemon-service-start-queues-task-and-creates-service-state`
- `remote-testdaemon-status-reuses-running-service`
- `remote-testdaemon-command-reuses-service-pid`
- `remote-testdaemon-service-recovery-replaces-missing-process`
- `remote-testdaemon-service-stop-stops-owned-process`
- `remote-testdaemon-service-list-on-legion-shows-owner-task`

## J. Chrome Browser Integration Over The Service Layer

These tests are allowed to depend on `chrome src_v3`, but they should prove browser-specific behavior only after the shared service contract already passes with `testdaemon`.

Required tests:

- `chrome-status-uses-service-layer-contract`
- `chrome-command-reuses-existing-service-when-healthy`
- `chrome-browser-failure-surfaces-through-task-log`

## K. Failure, Timeout, And Recovery

Required tests:

- `task-panic-appears-as-error-and-nonzero-exit`
- `task-crash-appears-as-error-and-nonzero-exit`
- `task-timeout-produces-clear-task-error`
- `hung-task-can-be-killed-by-task-id`
- `missing-service-is-reconciled-on-next-command`
- `heartbeat-loss-marks-service-unhealthy`
- `heartbeat-loss-triggers-service-restart`

## L. Interleaving And Isolation

Required tests:

- `multiple-running-tasks-produce-interleaved-but-coherent-output`
- `one-failing-task-does-not-hide-other-successful-tasks`
- `task-log-remains-isolated-under-interleaving`
- `service-output-does-not-corrupt-unrelated-task-state`

## M. Config And Bootstrap

Required tests:

- `bootstrap-creates-valid-env-dialtone-json`
- `add-host-updates-mesh-config`
- `task-runtime-resolves-repo-roots-correctly`
- `task-runtime-uses-managed-go-and-bun`

## N. Logs And Observability

Required tests:

- `logs-task-subject-exists-for-every-task`
- `logs-service-subject-exists-for-running-service`
- `logfilter-level-error-captures-task-failures`
- `task-log-and-logs-stream-surface-the-same-core-events`

## O. Multi-Host And Isolation

Required tests:

- `tasks-on-grey-and-legion-keep-distinct-host-state`
- `same-command-on-two-hosts-gets-two-task-ids`
- `service-owner-task-is-host-specific`
- `host-filtered-task-list-returns-only-target-host`

## Explicit dialtone> Transcript Scenarios

## 1. Local One-Shot CLI Command

```text
host-name> /proc src_v1 emit shell-contract-check
dialtone> Request received.
dialtone> Task queued as task-20260327-help001.
dialtone> Task topic: task.task-20260327-help001
dialtone> Task log: ~/.dialtone/logs/task-20260327-help001.log
dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-20260327-help001 --lines 10
```

## 2. Remote SSH CLI Command

```text
host-name> /ssh src_v1 run --host grey --cmd hostname
dialtone> Request received.
dialtone> Task queued as task-20260327-ssh001.
dialtone> Task topic: task.task-20260327-ssh001
dialtone> Task log: ~/.dialtone/logs/task-20260327-ssh001.log
dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-20260327-ssh001 --lines 10
```

## 3. One-Shot Service Start

```text
host-name> /testdaemon src_v1 service --host legion --mode start --name demo
dialtone> Request received.
dialtone> Task queued as task-20260327-svc001.
dialtone> Task topic: task.task-20260327-svc001
dialtone> Task log: ~/.dialtone/logs/task-20260327-svc001.log
dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-20260327-svc001 --lines 10
```

## 4. Failure Still Queues As A Task

```text
host-name> /testdaemon src_v1 exit-code --host legion --code 17
dialtone> Request received.
dialtone> Task queued as task-20260327-fail001.
dialtone> Task topic: task.task-20260327-fail001
dialtone> Task log: ~/.dialtone/logs/task-20260327-fail001.log
dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-20260327-fail001 --lines 10
```

## 5. Interleaving

```text
host-name> /proc src_v1 sleep 20
dialtone> Request received.
dialtone> Task queued as task-20260327-sleep01.
dialtone> Task topic: task.task-20260327-sleep01
dialtone> Task log: ~/.dialtone/logs/task-20260327-sleep01.log

host-name> /ssh src_v1 run --host grey --cmd 'echo ready'
dialtone> Request received.
dialtone> Task queued as task-20260327-echo01.
dialtone> Task topic: task.task-20260327-echo01
dialtone> Task log: ~/.dialtone/logs/task-20260327-echo01.log

host-name> /ssh src_v1 run --host grey --cmd 'echo boom >&2; exit 17'
dialtone> Request received.
dialtone> Task queued as task-20260327-fail01.
dialtone> Task topic: task.task-20260327-fail01
dialtone> Task log: ~/.dialtone/logs/task-20260327-fail01.log

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
./dialtone.sh repl src_v3 test --filter shell-routed-command-autostarts-leader-when-missing
./dialtone.sh repl src_v3 test --filter shell-routed-command-reuses-running-leader
./dialtone.sh repl src_v3 test --filter interactive-command-index-lifecycle-contract
./dialtone.sh repl src_v3 test --filter interactive-nonzero-exit-lifecycle
./dialtone.sh repl src_v3 test --filter task-list-and-log-match-real-command
./dialtone.sh repl src_v3 test --filter interactive-task-attach-detach
./dialtone.sh proc src_v1 emit shell-contract-check
```

The narrowest useful first command is:

```bash
./dialtone.sh repl src_v3 test --filter shell-routed-command-autostarts-leader-when-missing
```

The best local control-plane loop after that is:

```bash
./dialtone.sh repl src_v3 test --filter shell-routed-command-autostarts-leader-when-missing,shell-routed-command-reuses-running-leader,interactive-command-index-lifecycle-contract,task-list-and-log-match-real-command
```

## Success Criteria

The migration is successful when all of these are true:

- every request returns a `task-id`
- every request gets a task topic and task log before PID assignment
- every one-shot CLI call returns immediately after printing the queued-task summary and log-inspection hint
- the top-level transcript is task-first everywhere
- the active CLI and REPL path render lowercase `dialtone>`
- the CLI depends only on task-first wording
- `task list`, `task show`, `task log`, and `task kill` are the standard operator tools
- local and remote work share the same task lifecycle semantics
- `chrome src_v3` is fully managed as REPL service state on `legion`
- service health and restart behavior are driven by heartbeats and NATS KV state
- the task log and NATS subjects provide durable, inspectable state for every task
