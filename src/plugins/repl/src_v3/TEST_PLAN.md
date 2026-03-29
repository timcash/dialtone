# REPL src_v3 Test Plan

## Purpose

This plan is intentionally focused on the biggest unfinished parts of the `repl src_v3` system.
It is not a broad product checklist.

The core claim we still need to prove is:

- `repl src_v3` is a task-first, service-aware control plane
- queued-by-default command dispatch and foreground-query exceptions behave consistently
- durable task and service state lives in NATS KV
- service health is driven by heartbeats, not by optimistic process assumptions
- local and remote services behave the same way through the same operator surface

Until the focused areas in this file are implemented and passing, do not treat Chrome, Cloudflare, robot, or public-edge success as proof that the REPL core is complete.

This plan is written against:

- [README.md](/C:/Users/timca/dialtone/README.md)
- [README.md](/C:/Users/timca/dialtone/src/plugins/repl/src_v3/README.md)

## The Big Missing Areas

These are the areas this plan now centers on:

1. The queue-vs-foreground dispatch contract is only partially proven today.
2. `testdaemon` now exists, but the generic fixture proof is still only partially implemented.
3. NATS KV task and service state is not yet fully proven as the source of truth.
4. Heartbeat-driven unhealthy detection and reconcile/restart behavior is not yet fully proven.
5. Remote service suites are not yet deep enough to prove the model on real hosts.
6. Legacy `subtone`-named suites still exist in the repo and should not remain part of the long-term proof surface.

If these areas are not covered by real implemented test code, the migration is not done.

## What Does Not Count As Enough

These do not substitute for the focused work above:

- a passing Chrome service flow
- a passing robot relay flow
- a passing public `rover-1.dialtone.earth` health check
- docs that describe desired behavior
- a few manual CLI smoke runs
- tests that still depend on process-first or `subtone` behavior

Those flows are useful integration checks, but they are not the core proof.

## Definition Of Done

The focused work in this file is complete only when all of these are true:

- queued commands return a task-first transcript and foreground queries return direct data
- queued and foreground paths both autostart or reuse the correct background leader
- `testdaemon` exists and is the generic fixture for shared service-control-plane tests
- task creation, updates, and lookup are proven against NATS KV state
- service desired state and observed state are proven against NATS KV state
- service heartbeats update observed state continuously
- missed heartbeats mark services unhealthy
- the leader reconcile loop restarts unhealthy or missing services when desired state says they should be running
- leader restart or failover can recover enough state from KV to keep the system coherent
- remote `testdaemon` suites on `legion` pass and prove the same semantics as local runs
- Chrome tests are layered on top of the above, not used instead of the above

## Priority Order

Work in this order:

1. Prove queue-vs-foreground dispatch and leader autostart or reuse
2. Build `testdaemon`
3. Prove task KV state
4. Prove service KV state
5. Prove heartbeat and reconcile
6. Prove remote service behavior with `testdaemon`
7. Remove or replace legacy `subtone` suites
8. Only then spend time on Chrome, robot, Cloudflare, and public-edge integrations

## Required Workflow

If you are on Windows, you must use the supported Dialtone workflow through `.\wsl-tmux.cmd`, not raw toolchain commands or direct WSL REPL/test runs:

```powershell
.\wsl-tmux.cmd clean-state
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 process-clean"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 format"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 build"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 test --filter <name>"
.\wsl-tmux.cmd read
```

When working on `testdaemon`, use the same workflow style:

```powershell
.\wsl-tmux.cmd "./dialtone.sh testdaemon src_v1 format"
.\wsl-tmux.cmd "./dialtone.sh testdaemon src_v1 build"
.\wsl-tmux.cmd "./dialtone.sh testdaemon src_v1 test"
```

If bootstrap or runtime code changed, restart the background processes first:

```powershell
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 process-clean"
```

## Focus Area 0: Dispatch Contract

### Goal

Before the deeper KV and service work, prove the public shell contract:

- queued by default
- foreground only for explicit query/operator commands
- both paths autostart or reuse the correct background leader
- queued output is short and task-first
- foreground output returns direct data instead of a queued transcript

### Required Tests

- `shell-routed-command-autostarts-leader-when-missing`
- `shell-routed-command-reuses-running-leader`
- `shell-foreground-query-autostarts-leader-and-prints-direct-output`
- `interactive-command-index-lifecycle-contract`
- `interactive-command-index-emits-task-queue-lines`

### Required CLI Assertions

Queued path:

```text
dialtone> Request received.
dialtone> Task queued as task-...
dialtone> Task topic: task.task-...
dialtone> Task log: ...
dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-... --lines 10
```

Foreground path:

```text
No active managed processes.
```

These tests should fail if:

- a query command unexpectedly prints the queued task transcript
- a normal command blocks waiting for full lifecycle
- the shell path starts a second leader instead of reusing a healthy one
- the output regresses to legacy `subtone` terms or back to `room` wording

## Focus Area 1: testdaemon Fixture

### Goal

We need one simple fixture that proves the generic task and service model without depending on Chrome or any other plugin.

### Required Location

- `src/plugins/testdaemon/src_v1`

### Required Operator Surface

These commands should exist and be stable enough for the REPL suites:

```bash
./dialtone.sh testdaemon src_v1 build
./dialtone.sh testdaemon src_v1 test
./dialtone.sh testdaemon src_v1 run --mode once
./dialtone.sh testdaemon src_v1 service --mode start --name demo
./dialtone.sh testdaemon src_v1 service --mode status --name demo
./dialtone.sh testdaemon src_v1 service --mode stop --name demo
./dialtone.sh testdaemon src_v1 emit-progress --steps 5
./dialtone.sh testdaemon src_v1 sleep --seconds 10
./dialtone.sh testdaemon src_v1 exit-code --code 17
./dialtone.sh testdaemon src_v1 panic
./dialtone.sh testdaemon src_v1 crash
./dialtone.sh testdaemon src_v1 hang
./dialtone.sh testdaemon src_v1 heartbeat --name demo
./dialtone.sh testdaemon src_v1 shutdown --name demo
```

### Required Fixture Capabilities

- can run as a one-shot task
- can run as a long-lived service
- emits logs through the shared logs path
- emits explicit progress lines
- publishes heartbeats on a predictable interval
- can stop heartbeats on command
- can exit cleanly
- can exit nonzero
- can panic or crash
- can hang
- can report host, PID, started time, and service name

### Required Tests

- `testdaemon-builds`
- `testdaemon-one-shot-command-emits-progress`
- `testdaemon-service-starts`
- `testdaemon-service-stops`
- `testdaemon-can-exit-nonzero`
- `testdaemon-can-panic`
- `testdaemon-can-hang`
- `testdaemon-heartbeats-while-running`
- `testdaemon-can-stop-heartbeats-without-exiting`

### Acceptance Criteria

Do not move on to the later areas until `testdaemon` is real and can drive the generic suites.

## Focus Area 2: Task State In NATS KV

### Goal

Task identity must be task-first and durable. PID is later runtime detail. The leader and operator commands must be reading and updating task state through KV, not reconstructing it from local process scans.

### Minimum Task Record Contract

Every task record should be provably able to answer:

- task id
- command
- args
- topic
- log path
- host
- desired mode
- current state
- PID if assigned
- exit code if finished
- created time
- updated time

### Required Tests

- `task-submit-creates-kv-record-before-launch`
- `task-record-includes-task-id-command-topic-log-host`
- `task-record-exists-before-pid-assignment`
- `task-record-updates-to-running-after-launch`
- `task-record-stores-pid-after-launch`
- `task-record-updates-to-exited-on-finish`
- `task-record-stores-exit-code-on-finish`
- `task-list-reads-from-kv`
- `task-show-reads-from-kv`
- `task-show-prefers-kv-over-live-registry-fields`
- `task-list-prefers-kv-over-live-registry-fields`
- `task-state-queries-follow-kv-over-finished-task-history`
- `task-log-by-task-id-still-works-after-task-exit`
- `queued-task-still-visible-after-leader-restart`
- `finished-task-still-visible-after-leader-restart`

### Required CLI Assertions

```text
dialtone> Request received.
dialtone> Task queued as task-...
dialtone> Task topic: task.task-...
dialtone> Task log: ...
```

These tests should fail if:

- the system leads with PID
- task lookup depends on PID identity
- task list is really just a process list
- a task disappears because the leader restarted

## Focus Area 3: Service State In NATS KV

### Goal

Services must be modeled as desired state plus observed state, not as "there happens to be a process."

### Minimum Service Record Contract

Every service record should be provably able to answer:

- service name
- host
- desired state
- observed state
- health
- owner task id
- current PID if any
- heartbeat timestamp
- restart count
- updated time

### Required Tests

- `service-start-creates-desired-running-state`
- `service-stop-clears-desired-running-state`
- `service-show-reads-from-kv`
- `service-list-reads-from-kv`
- `service-record-is-keyed-by-host-and-name`
- `service-record-includes-owner-task`
- `service-record-includes-heartbeat-time`
- `service-record-persists-across-leader-restart`

### Required Negative Coverage

- two hosts with the same service name must not overwrite each other
- a stale PID must not be treated as healthy service state
- a missing process with desired state `running` must not look healthy

## Focus Area 4: Heartbeats And Reconcile

### Goal

The most important unfinished behavioral proof is:

- healthy services keep publishing heartbeats
- heartbeat loss marks them unhealthy
- reconcile notices the unhealthy or missing service
- reconcile restarts it when desired state still says `running`

### Required Test Conditions

Use short test-only timing values so these suites are fast and deterministic.

### Required Tests

- `service-heartbeat-updates-observed-state`
- `service-heartbeat-keeps-health-healthy`
- `missed-heartbeat-marks-service-unhealthy`
- `leader-restarts-service-after-heartbeat-loss`
- `leader-does-not-restart-healthy-service`
- `manual-stop-prevents-reconcile-restart`
- `missing-process-without-heartbeats-triggers-restart`
- `leader-restart-resumes-heartbeat-monitoring`
- `restarted-service-gets-new-pid-but-same-service-identity`

### Required Evidence

For each test above, prove all three layers:

- top-level `dialtone>` transcript
- KV state transition
- task log / service log evidence

### Required Failure Cases

- hanging service stops heartbeating
- crashed service stops heartbeating
- heartbeat publisher is alive but service process is gone
- reconcile loop restarts the wrong host or wrong service name

## Focus Area 5: Remote Service Suites

### Goal

The local model is not enough. We need to prove the same service semantics on real hosts.

### Required Remote Targets

- `legion`: primary remote service target

### Remote Service Tests That Must Use testdaemon

- `remote-testdaemon-service-start-on-legion`
- `remote-testdaemon-service-status-on-legion`
- `remote-testdaemon-service-list-on-legion`
- `remote-testdaemon-service-show-on-legion`
- `remote-testdaemon-service-stop-on-legion`
- `remote-testdaemon-service-heartbeat-on-legion`
- `remote-testdaemon-heartbeat-loss-restarts-on-legion`
- `remote-testdaemon-command-reuses-existing-service-on-legion`
- `remote-testdaemon-owner-task-is-correct-on-legion`
- `remote-testdaemon-log-and-kv-state-match-on-legion`

### Required Rule

Chrome must not be the thing proving that remote service semantics work.
Chrome may have its own later suite, but the shared remote service proof must come from `testdaemon`.

## Focus Area 6: Env Root And Leader Isolation

### Goal

This matters because the REPL leader is backgrounded automatically now.
We need to prove that launch-folder default config and explicit `--env` runs do not bleed into each other.

### Required Tests

- `launch-folder-env-is-used-when-env-flag-is-omitted`
- `explicit-env-switches-to-alternate-config-root`
- `leader-autostarts-under-default-env-root`
- `leader-autostarts-under-explicit-env-root`
- `default-env-and-alt-env-do-not-share-task-state`
- `default-env-and-alt-env-do-not-share-service-state`
- `task-list-in-alt-env-does-not-show-default-env-tasks`

## Focus Area 7: Remove Legacy Process-First Suites

### Goal

The repo still contains legacy `subtone`-named suites and helpers.
Those are useful migration clues, but they should not remain part of the final proof surface for `repl src_v3`.

### Current Legacy Areas To Replace

- `src/plugins/repl/src_v3/test/08_subtone_observability`
- `src/plugins/repl/src_v3/test/09_subtone_attach`

### Required Work

- replace `subtone` language with `task` and `service` in suite names, reports, and helper text
- move any still-useful assertions into the task-first suites
- remove or retire suites that only prove PID-first or process-first behavior
- make sure the final focused suite set reads like the public operator model

## Explicit Must-Pass Transcript Shapes

### One-Shot Command

```text
host-name> /proc src_v1 emit shell-contract-check
dialtone> Request received.
dialtone> Task queued as task-20260327-abc123.
dialtone> Task topic: task.task-20260327-abc123
dialtone> Task log: ~/.dialtone/logs/task-20260327-abc123.log
dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-20260327-abc123 --lines 10
```

### Interactive REPL

```text
dialtone> Connected to repl.topic.index via ...
dialtone> Leader online on DIALTONE-SERVER (topic=repl.topic.index ...)
dialtone> Shared REPL session ready on topic index.
```

### Service Start

```text
host-name> /testdaemon src_v1 service --host legion --mode start --name demo
dialtone> Request received.
dialtone> Task queued as task-20260327-svc001.
dialtone> Task topic: task.task-20260327-svc001
dialtone> Task log: ~/.dialtone/logs/task-20260327-svc001.log
dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-20260327-svc001 --lines 10
```

### Heartbeat Recovery

Expected later lifecycle shape:

```text
dialtone> Task task-20260327-svc001 assigned pid 25516 on legion.
dialtone> testdaemon service demo on legion is healthy.
dialtone> WARNING service demo on legion missed heartbeat.
dialtone> Reconcile restarting service demo on legion.
dialtone> Task task-20260327-svc009 assigned pid 25599 on legion.
dialtone> testdaemon service demo on legion is healthy.
```

## Suites To Implement

Only these suites are the current priority:

- `00_dispatch_contract`
- `01_testdaemon_fixture`
- `02_task_kv_state`
- `03_service_kv_state`
- `04_heartbeat_and_reconcile`
- `05_remote_service_legion`
- `06_env_root_isolation`
- `07_legacy_suite_replacement`

These may exist later, but they are not the main gate right now:

- Chrome browser integration
- robot/public-edge integration
- Cloudflare/public-host integration
- remote SSH and deploy integration beyond what the shared service model needs
- workstation-specific WSL diagnostics beyond what is needed for the core suites

For now, the default `./dialtone.sh repl src_v3 test` registry should stay focused on the core proof surface and exclude the longer SSH- and Cloudflare-dependent integration suites.
That includes the explicit SSH and Cloudflare suites plus the older SSH-fixture observability and attach slices that still depend on remote hosts.
Treat those flows as separate opt-in integration coverage until the core `testdaemon`, KV, and reconcile work is complete.

### Default Registry Today

The default `./dialtone.sh repl src_v3 test` gate should currently contain only:

- `00_process_manager`
- `01_tmp_workspace`
- `02_cli_help`
- `03_bootstrap_config`
- `04_repl_help_ps`
- `07_tsnet_ephemeral`
- `10_repl_logging_contract`
- `11_testdaemon_fixture`
- `12_task_kv_state`

These suites are the current opt-in integration coverage and should not be part of the default gate while the core proof surface is still stabilizing:

- `05_ssh_wsl`
- `06_cloudflare_tunnel`
- `08_task_observability`
- `09_task_attach`

## Commands To Keep Using During The Migration Loop

Core REPL loop:

```bash
./dialtone.sh repl src_v3 process-clean
./dialtone.sh repl src_v3 format
./dialtone.sh repl src_v3 build
./dialtone.sh repl src_v3 test --filter shell-routed-command-autostarts-leader-when-missing
./dialtone.sh repl src_v3 test --filter shell-routed-command-reuses-running-leader
./dialtone.sh repl src_v3 test --filter shell-foreground-query-autostarts-leader-and-prints-direct-output
./dialtone.sh repl src_v3 test --filter interactive-command-index-lifecycle-contract
```

Focused fixture loop:

```bash
./dialtone.sh testdaemon src_v1 format
./dialtone.sh testdaemon src_v1 build
./dialtone.sh testdaemon src_v1 test
```

Focused service-model loop once the suites exist:

```bash
./dialtone.sh repl src_v3 test --filter testdaemon-builds
./dialtone.sh repl src_v3 test --filter task-submit-creates-kv-record-before-launch
./dialtone.sh repl src_v3 test --filter service-start-creates-desired-running-state
./dialtone.sh repl src_v3 test --filter missed-heartbeat-marks-service-unhealthy
./dialtone.sh repl src_v3 test --filter leader-restarts-service-after-heartbeat-loss
./dialtone.sh repl src_v3 test --filter remote-testdaemon-service-start-on-legion
./dialtone.sh repl src_v3 test --filter default-env-and-alt-env-do-not-share-task-state
```

## Current Verified Baseline

These are useful smoke checks that already passed recently, but they are not enough to declare the focused work complete:

- `./dialtone.sh testdaemon src_v1 format`
- `./dialtone.sh testdaemon src_v1 build`
- `./dialtone.sh testdaemon src_v1 test`
- `./dialtone.sh repl src_v3 format`
- `./dialtone.sh repl src_v3 build`
- `./dialtone.sh repl src_v3 test --filter testdaemon`
- `./dialtone.sh repl src_v3 test --filter task-submit,task-record-includes,task-record-exists`
- `./dialtone.sh repl src_v3 test --filter task-record-updates-to-running,task-record-stores-pid,task-record-updates-to-exited`
- `./dialtone.sh repl src_v3 test --filter task-record-stores-exit-code-on-finish,task-list-reads-from-kv,task-show-reads-from-kv,task-log-by-task-id-still-works-after-task-exit,queued-task-still-visible-after-leader-restart,finished-task-still-visible-after-leader-restart`
- `./dialtone.sh repl src_v3 test --filter shell-routed-command-autostarts-leader-when-missing`
- `./dialtone.sh repl src_v3 test --filter shell-routed-command-reuses-running-leader`
- `./dialtone.sh repl src_v3 test --filter shell-foreground-query-autostarts-leader-and-prints-direct-output`
- `./dialtone.sh repl src_v3 test --filter interactive-command-index-lifecycle-contract`
- `./dialtone.sh repl src_v3 test`

Again: these are helpful, but they do not replace the focused areas above.

## Current Gaps In The Repo

Be honest about the current state while working this plan:

- `src/plugins/testdaemon/src_v1` now exists as the generic local fixture, and the focused local REPL slice now covers build/progress/failure/service basics against it, but KV/reconcile/remote proof is still missing
- the local task-KV slice now covers the full Focus Area 2 proof set plus direct source-of-truth override checks: queued-record creation, canonical fields, pre-PID visibility, running transition, PID persistence, done transition, exit-code persistence, KV-backed task list/show, KV-over-registry field preference, KV-over-history state preference, durable task log lookup by task id, and queued/finished visibility after leader restart
- legacy `subtone` suite directories still exist under `src/plugins/repl/src_v3/test`
- current dispatch and logging tests are useful, but they do not yet prove KV-backed task or service state
- current Chrome, robot, Cloudflare, and public-edge wins are integration proof only, not the core gate

## Final Success Criteria

This work is successful only when:

- `testdaemon` is the generic proof fixture
- KV-backed task state is proven
- KV-backed service state is proven
- heartbeat loss and reconcile/restart are proven
- remote service behavior is proven on `legion`
- env-root leader isolation is proven
- Chrome and other plugin tests are clearly layered on top of that core, not used instead of it
