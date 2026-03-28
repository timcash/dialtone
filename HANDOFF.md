# Handoff For The Next LLM Agent

This repository is migrating `repl src_v3` and `chrome src_v3` to a pure task-first system.

The public direction is:

- no public `subtone` language
- no PID-first operator surface
- no old CLI or NATS interaction model as the target
- task and service state live in the REPL embedded NATS
- `dialtone.sh` is a thin client that submits tasks or opens the REPL
- `dialtone>` output should match the task-first examples in the docs

If you see old `subtone` names in code, treat them as migration debt to remove, not behavior to preserve.

## Read These Docs First

Read these in this order before changing more code:

1. [README.md](README.md)
   Focus on:
   - `Using dialtone.sh`
   - `Working With Plugins`
   - `Running The REPL`
   - `Inspecting Tasks And Services`
   - `NATS-First Logs`
   - `Windows Development`
2. [src/plugins/README.md](src/plugins/README.md)
   Focus on:
   - `Generic Shell Workflow`
   - `Standard Plugin Layout`
   - `NATS Topic Usage`
3. [src/plugins/repl/src_v3/README.md](src/plugins/repl/src_v3/README.md)
   Focus on:
   - `Default Use`
   - `REPL Standards`
   - `Tasks, Services, And The Operator Surface`
   - `Windows To WSL Workflow`
   - `For LLM Agents`
4. [src/plugins/chrome/src_v3/README.md](src/plugins/chrome/src_v3/README.md)
   Focus on:
   - `REPL Integration`
   - `Core Model`
   - `Config And State`
   - `Logs And Observability`
   - `Hybrid Windows + WSL Workflow`
5. [src/plugins/repl/src_v3/DESIGN.md](src/plugins/repl/src_v3/DESIGN.md)
   Focus on:
   - `Target Design`
   - `Proposed NATS Model`
   - `Proposed Operator Query Surface`
   - `Proposed Task Model`
   - `Proposed Service Model`
   - `What Needs To Change`
   - `Suggested Refactor Order`
6. [src/plugins/repl/src_v3/TEST_PLAN.md](src/plugins/repl/src_v3/TEST_PLAN.md)
   Focus on:
   - `Public Contract From The Docs`
   - `Start Here: 8 Steps`
   - `WSL tmux Test Workflow`
   - `Execution Tiers`
   - `Test Categories`
   - `Explicit dialtone> Transcript Scenarios`
   - `End-To-End Commands To Use While Migrating`

## Working Agreement

Keep these rules steady:

- The docs are the contract.
- The target output is task-first `dialtone>` output.
- Store durable task and service state in the embedded REPL NATS.
- PID is runtime metadata, not identity.
- Task logs are task-id-first.
- Prefer task topics and task ids in tests and operator commands.
- Use `./dialtone.sh <plugin> <src_vN> format` after code edits to plugin code.
- Use focused tests one by one while migrating.
- Do not add new public `subtone-*` commands, old room-based names, or uppercase `DIALTONE>` transcript text.

## WSL tmux Workflow

Use [wsl-tmux.cmd](wsl-tmux.cmd) for commands that should be visible in the shared WSL tmux session.

Important behavior from experience:

- `wsl-tmux.cmd` queues commands if you send a new command before the previous one finishes.
- The safest loop is:
  1. send one command
  2. use `read` until you see the prompt again
  3. then send the next command
- Use `interrupt` if a command is stuck.
- Use `clean-state` to clear partial shell input and tmux history before starting a new visible sequence.

Useful commands:

```powershell
.\wsl-tmux.cmd help
.\wsl-tmux.cmd status
.\wsl-tmux.cmd read
.\wsl-tmux.cmd interrupt
.\wsl-tmux.cmd clean-state
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 process-clean"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 format"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 test --filter interactive-task-attach-detach"
```

The wrapper delegates to `wsl-tmux.ps1`, which:

- creates or reuses a tmux session
- defaults the WSL cwd from `env/dialtone.json` or `/home/user/dialtone`
- supports `send`, `read`, `status`, `interrupt`, `clean-state`, and `list`

## Syncing Windows And WSL

The working repo is on Windows at `C:\Users\timca\dialtone`.
The WSL test repo is typically `/home/user/dialtone`.

Important gotcha:

- the REPL bootstrap HTTP server serves a tarball snapshot created at startup
- after code changes, isolated bootstrap-based tests may still use old code until you restart the local REPL/bootstrap helper processes

Recommended sequence after important code changes:

```powershell
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 process-clean"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 format"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 test --filter <step-name>"
```

If a file needs to be copied explicitly into `/home/user/dialtone` before rerunning a visible test, prefer an exact byte-for-byte overwrite instead of `cp`, because stale WSL copies can survive when a previous command used non-clobber behavior:

```powershell
@'
from pathlib import Path
Path("/home/user/dialtone/src/plugins/repl/src_v3/go/repl/core_runtime.go").write_bytes(
    Path("/mnt/c/Users/timca/dialtone/src/plugins/repl/src_v3/go/repl/core_runtime.go").read_bytes()
)
'@ | wsl.exe python3 -
```

## Current Migration State

Recent confirmed progress:

- default REPL prompt is hostname-first, not `llm-codex`, unless `--user` is passed
- interactive routed command lifecycle is task-first:
  - `Request received.`
  - `Task queued as task-...`
  - `Task topic: task.task-...`
  - `Task log: .../task-....log`
  - `Task task-... assigned pid ...`
  - `Task task-... exited with code ...`
- one-shot shell-routed `./dialtone.sh <plugin> ...` now returns right after:
  - `Request received.`
  - `Task queued as task-...`
  - `Task topic: task.task-...`
  - `Task log: .../task-....log`
  - `To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-... --lines 10`
- active user-facing output is lowercase `dialtone>` and attached-task output is lowercase `dialtone:<task-id>`
- `dialtone>` formatting is now centralized through the shared logs formatter and REPL/shell helpers, instead of being spread through ad hoc `fmt.Fprintf(... "DIALTONE> ...")` calls
- the canonical REPL control point for top-level `dialtone>` index-topic output now lives in [src/plugins/repl/src_v3/go/repl/dialtone_output.go](src/plugins/repl/src_v3/go/repl/dialtone_output.go)
- task log and task registry files exist as first-class code paths
- `task list` and `task log --task-id ...` are active test targets
- `/task-attach --task-id ...` and `/task-detach` are active REPL test targets

Recent focused WSL commands and tests that passed:

- `./dialtone.sh repl src_v3 format`
- `./dialtone.sh repl src_v3 build`
- `./dialtone.sh repl src_v3 test --filter shell-routed-command-autostarts-leader-when-missing`
- `./dialtone.sh repl src_v3 test --filter shell-routed-command-reuses-running-leader`
- `interactive-command-index-lifecycle-contract`
- `interactive-nonzero-exit-lifecycle`
- `task-list-and-log-match-real-command`
- `interactive-task-attach-detach`

One recent passing shell-routed transcript looked like:

```text
legion> /proc src_v1 emit shell-contract-check
dialtone> Request received.
dialtone> Task queued as task-20260327-232649-000.
dialtone> Task topic: task.task-20260327-232649-000
dialtone> Task log: /home/user/.dialtone/logs/task-20260327-232649-000.log
dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-20260327-232649-000 --lines 10
```

One recent passing REPL transcript looked like:

```text
legion> /ssh src_v1 probe --host wsl --timeout 5s
dialtone> Request received.
dialtone> Task queued as task-20260327-221635-000-002.
dialtone> Task topic: task.task-20260327-221635-000-002
dialtone> Task log: /home/user/.dialtone/logs/task-20260327-221635-000-002.log
dialtone> Task task-20260327-221635-000-002 assigned pid 127579.
dialtone> Attached to task task-20260327-221635-000-002.
dialtone> ssh probe: checking transport/auth for wsl
dialtone> ssh probe: transport=ssh preferred=grey.shad-artichoke.ts.net
dialtone:task-20260327-221635-000-002> Probe target=wsl transport=ssh user=user port=22
dialtone> Detached from task task-20260327-221635-000-002.
dialtone> ssh probe: auth checks passed for wsl
dialtone> Task task-20260327-221635-000-002 exited with code 0.
```

## Files Already In Motion

These files have active migration work and are worth reviewing before editing nearby code:

- [src/dev.go](src/dev.go)
- [src/plugins/repl/src_v3/go/repl/core_runtime.go](src/plugins/repl/src_v3/go/repl/core_runtime.go)
- [src/plugins/repl/src_v3/go/repl/dialtone_output.go](src/plugins/repl/src_v3/go/repl/dialtone_output.go)
- [src/plugins/repl/src_v3/go/repl/task_commands_v3.go](src/plugins/repl/src_v3/go/repl/task_commands_v3.go)
- [src/plugins/repl/src_v3/go/repl/task_log_v3.go](src/plugins/repl/src_v3/go/repl/task_log_v3.go)
- [src/plugins/repl/src_v3/go/repl/task_registry_v3.go](src/plugins/repl/src_v3/go/repl/task_registry_v3.go)
- [src/plugins/repl/src_v3/go/repl/service_registry_v3.go](src/plugins/repl/src_v3/go/repl/service_registry_v3.go)
- [src/plugins/repl/src_v3/test/support/runtime.go](src/plugins/repl/src_v3/test/support/runtime.go)
- [src/plugins/repl/src_v3/test/10_repl_logging_contract/suite.go](src/plugins/repl/src_v3/test/10_repl_logging_contract/suite.go)
- [src/plugins/repl/src_v3/test/08_task_observability/suite.go](src/plugins/repl/src_v3/test/08_task_observability/suite.go)
- [src/plugins/repl/src_v3/test/09_task_attach/suite.go](src/plugins/repl/src_v3/test/09_task_attach/suite.go)
- [src/plugins/repl/src_v3/test/cmd/main.go](src/plugins/repl/src_v3/test/cmd/main.go)

Also note:

- old `subtone_*` registry/log files were replaced with task-first files
- some deleted legacy files may still appear in `git status` until the migration lands

## Suggested Next Steps

Follow [src/plugins/repl/src_v3/TEST_PLAN.md](src/plugins/repl/src_v3/TEST_PLAN.md) and keep working one focused test at a time.

Best next slices:

1. finish removing remaining public `subtone` and old room-based wording from tests, helper text, and runtime output, including the leader-autostart preamble
2. tighten `task list`, `task show`, `task log`, and `task kill` around NATS KV-backed task snapshots
3. make service desired and observed state explicit in `service list` and `service show`
4. add or finish the `testdaemon` fixture so generic service reconciliation is proven without Chrome-specific code
5. keep Tier 1 local task-runtime tests green before pushing deeper into remote coverage
6. use `grey` for real SSH and deploy validation
7. use `legion` for long-lived service reconciliation after the generic `testdaemon` path is stable

## Focused Test Loop

Use this loop repeatedly:

```powershell
.\wsl-tmux.cmd clean-state
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 process-clean"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 format"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 test --filter <step-name>"
.\wsl-tmux.cmd read
```

Good focused commands to keep using:

```powershell
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 test --filter shell-routed-command-autostarts-leader-when-missing"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 test --filter shell-routed-command-reuses-running-leader"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 test --filter interactive-command-index-lifecycle-contract"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 test --filter interactive-nonzero-exit-lifecycle"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 test --filter task-list-and-log-match-real-command"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 test --filter interactive-task-attach-detach"
```

Recent live operator checks worth repeating:

```powershell
.\wsl-tmux.cmd "./dialtone.sh proc src_v1 emit task-surface-after-clean"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 task show --task-id <task-id>"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 10"
```

## Things That Bit Me

- Sending a new `wsl-tmux.cmd` command before the prompt returns can queue confusing extra commands.
- Bootstrap-based isolated tests can run stale code if `process-clean` is skipped after local code changes.
- Proc-side worker log filename changes also need a full `./dialtone.sh repl src_v3 process-clean` before a new routed task will show the updated worker log path.
- When a focused test fails, inspect the task topic, task log, and operator commands before making a broad code change.
- When exact Windows-to-WSL sync matters, prefer a Python byte copy over `cp` so stale files do not survive in `/home/user/dialtone`.
- Keep the user-facing transcript short and task-first. Internal implementation details do not belong in the main `dialtone>` stream.

## Bottom Line

The contract is now the docs, especially the root [README.md](README.md), [src/plugins/repl/src_v3/README.md](src/plugins/repl/src_v3/README.md), [src/plugins/repl/src_v3/DESIGN.md](src/plugins/repl/src_v3/DESIGN.md), and [src/plugins/repl/src_v3/TEST_PLAN.md](src/plugins/repl/src_v3/TEST_PLAN.md).

Keep migrating toward:

- task-first transcripts
- embedded-NATS task and service state
- task-id-first logs and operator commands
- focused tmux-visible tests in WSL

If you are unsure what to do next, return to `TEST_PLAN.md`, pick one focused step, run it through `wsl-tmux.cmd`, and move the code until the transcript matches the docs.

## Resolved Decisions For The Next Pass

1. The documented shell path `./dialtone.sh <plugin> ...` should return right away. It should not block to mirror later lifecycle lines. It should print the queued task metadata, the `task-id`, and a helpful command to inspect the log, then return to the shell.

2. Remove everything about `subtone` from the public and migration-facing domain language. Use `task` as the main unit. Some tasks can run as a `service`, meaning they are long-lived daemon-style tasks.

3. The REPL leader should keep a heartbeat on any service task. If the service stops sending heartbeats, the leader should treat that as unhealthy and reconcile/restart it.

4. Durable task and service state should be stored in the NATS KV store.
