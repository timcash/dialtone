# Mods System (`src/mods`)

This directory is the versioned mods system for Dialtone.

This file is the single system guide for:

- how to structure a mod
- how to run the visible shell workflow
- how SQLite tracks state and tests
- how an LLM should work inside the mods system

## Mod CLI Contract

Every real mod version in `src/mods/<mod-name>/<version>/` should have a small Go CLI wrapper at:

```text
src/mods/<mod-name>/<version>/cli/main.go
```

That CLI wrapper is the contract between the mod and [src/mods.go](/Users/user/dialtone/src/mods.go).

It should handle the basic development commands for the mod:

- `install`
- `build`
- `format`
- `test`

Those command names are part of the shared contract. Their exact behavior can vary by mod.

Why this exists:

- it gives every mod one consistent Go entrypoint for `src/mods.go`
- it lets an LLM use plain `./dialtone_mod <mod> <version> ...` commands instead of raw `gofmt`, `go build`, `go run`, or ad hoc shell snippets
- it keeps the basic development workflow uniform across mods

Hard rules:

- this contract applies to every mod, including `dialtone v1`
- the CLI wrapper is for hooking the mod into `src/mods.go` and for exposing the basic development commands first
- every mod should expose `install`, `build`, `format`, and `test`, but each mod may implement those commands in its own way
- builds should write outputs under `<repo-root>/bin/mods/<mod-name>/<version>/` so the `bin` tree mirrors the `src/mods` tree
- a runtime binary may execute outside Nix, but its `install`, `build`, `format`, and `test` flows should still be driven through the mod CLI wrapper from a Nix shell
- if a mod needs extra runtime commands, add them after the basic `install|build|format|test` contract is in place

## LLM Start Here

If you are a new LLM working in `src/mods`, use this workflow first.

Goal:

- keep `shell v1` at the center
- keep SQLite as the source of truth
- keep only a tiny bootstrap/report stub outside Nix
- run all visible tests in `dialtone-view`
- let plain `./dialtone_mod ...` commands queue into SQLite and run in `dialtone-view`
- expect routed commands to return immediately with a `command_id`
- write the Go test first, then change code, then rerun the test
- keep the local tmux control plane on the repo-root `.tmux.conf` when present

### End-to-End Test Flow

```sh
# 1. Run the small preflight first.
# This verifies the shell/SQLite basics before you start the UI workflow.
./dialtone_mod shell v1 test-basic

# 2. Start the visible local workflow.
# This should open one Ghostty window with one tmux session split into:
# left  = codex-view
# right = dialtone-view
./dialtone_mod shell v1 start --run-tests=false

# 3. Confirm the shell state from SQLite.
# Read state through the shell mod, not by manually inspecting tmux first.
./dialtone_mod shell v1 state --full

# 4. Plain dialtone_mod commands should stay plain.
# No `ENV=... ./dialtone_mod ...` prefixes are needed.
./dialtone_mod mods v1 db graph --format outline

# 4b. Inspect a routed command later by row id.
./dialtone_mod shell v1 status --row-id <command_id> --full --sync=false

# 5. Run one focused visible test for the mod you are changing.
# Always prefer recursive package tests so both the version-root smoke test
# and the CLI tests run together.
./dialtone_mod shell v1 run --wait-seconds 120 \
  "clear && cd /Users/user/dialtone/src && go test ./mods/<mod-name>/<version>/..."

# 6. After the focused test passes, run the broader visible suite.
./dialtone_mod shell v1 test-all --wait-seconds 240
```

### Development Loop

1. Read this file and the target mod README.
2. Write or update the Go test first.
3. Run the focused test visibly in `dialtone-view`.
4. Make the code change.
5. Rerun the same focused test in `dialtone-view`.
6. If the mod is interactive, verify its visible workflow through `shell v1`.
7. Run `./dialtone_mod shell v1 test-all --wait-seconds 240` before you finish.
8. Update the target mod README so `## Test Results` is the last section.

### Rules

- Do not use host Go.
- Do not run ad hoc `go test` outside the Nix-backed shell workflow.
- Prefer `./dialtone_mod <mod> <version> install|build|format|test` over raw `gofmt`, `go build`, `go test`, or `go run` when the mod wrapper exists.
- Prefer `./dialtone_mod shell v1 run ...` for focused work.
- Prefer `./dialtone_mod shell v1 test-all` for the full visible sweep.
- Do not prefix routed mod commands with custom environment variables.
- Treat the `command_id` returned by routed commands as the durable handle.
- Treat `ghostty v1`, `tmux v1`, and `codex v1` as backend mods behind `shell v1`.
- Read state from SQLite through the shell/mods commands when possible.
- Keep local tmux session bootstrap and pane control on the repo-root `.tmux.conf` when present.

### Fast Reference

```sh
# focused package test
./dialtone_mod shell v1 run --wait-seconds 120 \
  "clear && cd /Users/user/dialtone/src && go test ./mods/<mod-name>/<version>/..."

# shell-specific test
./dialtone_mod shell v1 test

# full visible suite
./dialtone_mod shell v1 test-all --wait-seconds 240

# inspect SQLite-backed shell state
./dialtone_mod shell v1 state --full
./dialtone_mod shell v1 status --row-id <command_id> --full --sync=false
./dialtone_mod shell v1 events --limit 20

# inspect the DAG / test plan
./dialtone_mod mods v1 db graph --format outline
./dialtone_mod mods v1 db test-plan

# full end-to-end control-plane check
./dialtone_mod test v1 start
```

## Core Model

The preferred local workflow is:

- one Ghostty window
- one Ghostty tab
- one tmux session: `codex-view`
- left pane: `codex-view:0:0`
- right pane: `codex-view:0:1` titled `dialtone-view`

Roles:

- `dialtone_mod`
  Thin bootstrap/enqueue/report client. It bootstraps the local install, builds the standalone `dialtone` binary when needed, ensures it is running, queues work in SQLite, and returns immediately with a `command_id` plus process/status data for the agent.
- `dialtone`
  Standalone control-plane/process-manager binary started by `dialtone_mod`. It owns the SQLite database, queue state, and worker supervision outside Nix.
- `codex-view`
  Prompt and reasoning pane
- `dialtone-view`
  Visible command/test pane with the long-lived SQLite shell worker. This is where routed plain `./dialtone_mod ...` commands actually run.
- SQLite
  Source of truth for targets, queue rows, command status, outputs, snapshots, DAG, and test state
- `shell v1`
  High-level orchestrator for the visible workflow
- `test v1`
  End-to-end acceptance harness for the whole client/daemon/worker/SQLite flow

Treat `ghostty v1`, `tmux v1`, and `codex v1` as supporting mods behind `shell v1`.

## Architecture Goal

The architecture should be:

- `dialtone_mod`
  Tiny user-facing entrypoint. It should bootstrap the local install, ensure `dialtone` is running, queue routed work, print useful state immediately, and return fast.
- `dialtone`
  One long-lived control-plane process outside Nix. It should own the SQLite database, queue lifecycle, process supervision, health/heartbeat state, and fast status reporting.
- `dialtone-view`
  One long-lived worker pane inside the shared Nix shell. All routed plain `./dialtone_mod ...` commands should execute here, visibly.
- `codex-view`
  Prompt/reasoning pane. Routed mod commands should never actually execute here.
- SQLite
  Source of truth for all durable state: targets, queue rows, process state, PID, exit code, runtime, output, pane snapshots, protocol runs, and test results.

Hard rules:

- plain routed commands stay plain: `./dialtone_mod ...`
- no `ENV=... ./dialtone_mod ...` prefixes
- no routed mod command should execute in the caller terminal
- `./dialtone_mod` should return quickly with `command_id`, state, and inspection hints
- background state should live in SQLite, not in ad hoc shell environment

Current implementation:

- this architecture is mostly in place now
- `dialtone_mod` is still a shell wrapper
- the standalone daemon now lives in `src/mods/dialtone/v1`
- `dialtone_mod` now builds and `exec`s that standalone binary for `dialtone v1 ensure|serve|status|queue`
- `dialtone v1 ensure` replaces a legacy `dialtone_mod __dialtone serve` daemon with the standalone binary when one is still running
- the worker is `shell v1 serve` in `dialtone-view`
- fast `shell v1 status|state` reads now delegate to `dialtone v1 status`
- local `tmux` control-plane commands now use the repo-root `.tmux.conf` when it exists

Current `test v1 start` coverage:

- it proves the prompt is delivered to `codex-view`
- it proves the harness waits for the Codex CLI banner before prompt submission
- it proves Codex itself can run one prompted plain routed `./dialtone_mod ...` command from `codex-view`
- it proves that Codex-initiated SQLite row is created after the prompt row and still executes in `dialtone-view`
- it proves routed plain `./dialtone_mod ...` commands execute in `dialtone-view`
- it proves the worker survives long-running, failure, invalid-input, recovery, and background cases
- it still uses the harness to queue the larger deterministic scenario matrix after the first Codex-initiated command

## Control Plane Cleanup

The post-split cleanup is now in place:

- the dead Bash daemon/status/queue implementation is gone from `dialtone_mod`
- `src/mods.go` no longer keeps its own routed-command queue/report path
- routed commands from the Go dispatcher now delegate to `dialtone v1 queue`
- the user-facing command stays the same: `./dialtone_mod ...`
- `dialtone` still runs outside Nix and the worker still runs in `dialtone-view`
- the SQLite schema and fast status contract did not change

The next acceptance-test step is:

- keep `./dialtone_mod test v1 start` as the visible control-plane proof
- keep the first Codex-initiated routed command as the gating proof that `codex-view` and `dialtone-view` are cooperating correctly
- move more of the scenario matrix from harness-queued commands to Codex-initiated commands
- prompt Codex to run a unique list of plain `./dialtone_mod ...` commands with mixed expected outcomes
- wait for new SQLite `shell_bus` rows after prompt submission
- prove those rows still target `dialtone-view`
- record enough metadata to tell Codex-initiated rows from harness-generated rows

## Test Commands

Use these commands to test the system.

Bootstrap and workflow:

```sh
# preflight for sqlite + shell packages
./dialtone_mod shell v1 test-basic

# ensure the visible workflow exists
./dialtone_mod shell v1 start --run-tests=false

# end-to-end acceptance test for the whole control plane
./dialtone_mod test v1 start
```

Focused and broad visible tests:

```sh
# run one focused package visibly in dialtone-view
./dialtone_mod shell v1 run --wait-seconds 120 \
  "clear && cd /Users/user/dialtone/src && go test ./mods/<mod-name>/<version>/..."

# run the shell module package tests visibly
./dialtone_mod shell v1 test

# run the full visible mod sweep
./dialtone_mod shell v1 test-all --wait-seconds 240
```

Inspect state after a run:

```sh
# cached overall system view
./dialtone_mod shell v1 state --full

# inspect one routed command by durable row id
./dialtone_mod shell v1 status --row-id <command_id> --full --sync=false

# read the prompt pane through SQLite snapshots
./dialtone_mod shell v1 read --role prompt --full

# read the command pane through SQLite snapshots
./dialtone_mod shell v1 read --role command --full

# recent shell bus rows
./dialtone_mod shell v1 events --limit 20

# protocol runs recorded by the end-to-end harness
./dialtone_mod mods v1 db protocol-runs --limit 10
./dialtone_mod mods v1 db protocol-events --run <protocol_run_id>
```

DB / graph checks:

```sh
# mod graph from SQLite
./dialtone_mod mods v1 db graph --format outline

# current SQLite-backed state values
./dialtone_mod mods v1 db state

# derived test plan
./dialtone_mod mods v1 db test-plan
```

## Important Files

These files matter most for the control-plane architecture:

- [dialtone_mod](/Users/user/dialtone/dialtone_mod)
  Thin entrypoint that bootstraps the repo and delegates to the standalone `dialtone` binary
- [src/mods/dialtone/v1/main.go](/Users/user/dialtone/src/mods/dialtone/v1/main.go)
  Standalone control-plane daemon and fast SQLite-backed status/report path
- [src/mods.go](/Users/user/dialtone/src/mods.go)
  Main Go mod dispatcher used inside Nix; routed commands now delegate to `dialtone v1 queue`
- [src/internal/modstate/modstate.go](/Users/user/dialtone/src/internal/modstate/modstate.go)
  SQLite schema, queue helpers, protocol run/event persistence
- [src/mods/shared/sqlitestate/sqlitestate.go](/Users/user/dialtone/src/mods/shared/sqlitestate/sqlitestate.go)
  Shared SQLite path and state-key definitions
- [src/mods/shared/dispatch/dispatch.go](/Users/user/dialtone/src/mods/shared/dispatch/dispatch.go)
  Routing decisions and shell intent encoding
- [src/mods/shared/router/router.go](/Users/user/dialtone/src/mods/shared/router/router.go)
  Queueing helpers and worker-health helpers
- [src/mods/shell/v1/cli/main.go](/Users/user/dialtone/src/mods/shell/v1/cli/main.go)
  Workflow bootstrap, `dialtone-view` worker, shell status/read/events commands
- [src/mods/test/v1/cli/main.go](/Users/user/dialtone/src/mods/test/v1/cli/main.go)
  End-to-end acceptance harness for the current architecture
- [src/mods/shell/v1/README.md](/Users/user/dialtone/src/mods/shell/v1/README.md)
  Operator guide for the shell workflow
- [src/mods/test/v1/README.md](/Users/user/dialtone/src/mods/test/v1/README.md)
  Operator guide for the end-to-end harness

## Current Handoff Status

Another LLM should assume this current state:

- the target architecture is mostly working now
- the standalone daemon now lives in [src/mods/dialtone/v1/main.go](/Users/user/dialtone/src/mods/dialtone/v1/main.go)
- [dialtone_mod](/Users/user/dialtone/dialtone_mod) builds and delegates to that binary for `dialtone v1 ensure|serve|status|queue`
- `dialtone v1 ensure` will replace a legacy `dialtone_mod __dialtone serve` process so the active daemon becomes `/Users/user/.dialtone/bin/dialtone serve`
- the visible worker is still [src/mods/shell/v1/cli/main.go](/Users/user/dialtone/src/mods/shell/v1/cli/main.go) running `shell v1 serve` in `dialtone-view`
- fast `status` and `state` reads now come from `dialtone v1 status`, usually through the wrapper fast path
- direct routed-command handling in [src/mods.go](/Users/user/dialtone/src/mods.go) now also delegates to `dialtone v1 queue`
- further cleanup is optional polish, not a structural requirement

Before changing the control plane, rerun these first:

```sh
./dialtone_mod shell v1 test-basic
./dialtone_mod test v1 start
```

Use the end-to-end harness result as the acceptance check:

- it should submit a prompt to `codex-view`
- it should queue a plain routed `./dialtone_mod mods v1 db graph --format outline`
- the command should run in `dialtone-view`
- SQLite should record protocol events, pane snapshots, `pid`, `exit_code`, and `runtime_ms`
- the final stdout should end with `test_result	passed`

Latest validated inspection pattern:

```sh
./dialtone_mod test v1 start
./dialtone_mod shell v1 status --row-id <command_row_id> --full --sync=false
./dialtone_mod mods v1 db protocol-events --run <protocol_run_id>
```

## Standard Layout

Every Go-backed mod version should use this shape:

```text
src/mods/<mod-name>/<version>/
├── README.md
├── mod.json
├── nix.packages            # optional
├── main_test.go            # version-root smoke/layout test
└── cli/
    ├── main.go             # runnable CLI entrypoint
    └── main_test.go        # feature/unit tests for the CLI
```

Rules:

- the runnable Go entrypoint lives in `cli/main.go`
- `cli/main.go` should expose the standard `install`, `build`, `format`, and `test` commands for the mod, even when their exact behavior differs by mod
- version-root `main_test.go` is a smoke/layout contract test
- CLI behavior tests live in `cli/main_test.go`
- builds should write outputs under `<repo-root>/bin/mods/<mod-name>/<version>/`
- do not put the runnable mod CLI entrypoint at the version root

## SQLite

The shared state database is:

```text
~/.dialtone/state.sqlite
```

SQLite stores:

- bootstrap status and log path
- the mod DAG
- test topology and test steps
- tmux prompt and command targets
- shell bus queue rows
- pane snapshots
- protocol and test results

Important target rows:

- `tmux.prompt_target = codex-view:0:0`
- `tmux.target = codex-view:0:1`

## Nix

Use one shared Nix shell:

- `default`

Rules:

- never use host Go
- load Nix before running Go commands
- visible tests in `dialtone-view` should reuse the already-running `default` shell
- avoid per-mod shell churn unless there is a real runtime reason

The current shared toolchain in the visible shell includes:

- Go 1.25.5
- tmux
- sqlite
- openssh
- expect

## Quick Start

```sh
# 1. Run the small SQLite/shell preflight first.
./dialtone_mod shell v1 test-basic

# 2. Start the visible Ghostty + tmux workflow.
./dialtone_mod shell v1 start --run-tests=false

# 3. Inspect the shell state from SQLite.
./dialtone_mod shell v1 state --full

# 4. Run one visible Go test command in dialtone-view.
./dialtone_mod shell v1 run --wait-seconds 120 \
  "clear && cd /Users/user/dialtone/src && go test ./mods/shell/v1/..."

# 5. Run the full visible suite in dialtone-view.
./dialtone_mod shell v1 test-all --wait-seconds 240
```

## LLM Workflow

LLMs should follow this loop:

1. Read this file and the target mod README.
2. Write the Go test first.
3. Run that test visibly in `dialtone-view`.
4. Make the code change.
5. Rerun the same visible test.
6. Only after the focused test is green, run the broader visible sweep if needed.
7. Update the target mod README so `## Test Results` is the last section.

Preferred visible commands:

```sh
# focused package run
./dialtone_mod shell v1 run --wait-seconds 120 \
  "clear && cd /Users/user/dialtone/src && go test ./mods/<mod-name>/<version>/..."

# shell preflight
./dialtone_mod shell v1 test-basic

# shell package
./dialtone_mod shell v1 test

# full visible sweep
./dialtone_mod shell v1 test-all --wait-seconds 240
```

## Testing Contract

All mod versions should have Go tests.

At minimum:

- one version-root smoke/layout test in `main_test.go`
- one CLI feature test file in `cli/main_test.go`

Preferred recursive package command:

```sh
go test ./mods/<mod-name>/<version>/...
```

That should cover:

- the version-root smoke/layout test
- the CLI feature tests

SQLite test plans should use the same recursive form.

## README Contract

Each `src/mods/<mod>/<version>/README.md` should contain, in this order when practical:

- `## Quick Start`
- `## DIALTONE>` when the mod is interactive
- `## Dependencies`
- `## Test Results`

`## Test Results` must be the last section.

It should record:

- the visible command used
- the most recent accepted result
- any important caveat that still matters

## New Mod Checklist

```sh
# 1. Create the standard layout.
mkdir -p src/mods/my_mod/v1/cli

# 2. Add the CLI entrypoint.
$EDITOR src/mods/my_mod/v1/cli/main.go

# 3. Add CLI tests first.
$EDITOR src/mods/my_mod/v1/cli/main_test.go

# 4. Add the version-root smoke/layout test.
$EDITOR src/mods/my_mod/v1/main_test.go

# 5. Add docs and manifest.
$EDITOR src/mods/my_mod/v1/README.md
$EDITOR src/mods/my_mod/v1/mod.json

# 6. Run the visible preflight.
./dialtone_mod shell v1 test-basic

# 7. Start the visible shell workflow.
./dialtone_mod shell v1 start --run-tests=false

# 8. Run the new mod tests visibly.
./dialtone_mod shell v1 run --wait-seconds 120 \
  "clear && cd /Users/user/dialtone/src && go test ./mods/my_mod/v1/..."
```

## Current Status

Latest accepted visible run:

- focused visible packages:
  - `mods/dialtone/v1`
  - `mods/shared/dispatch`
  - `mods/shared/router`
  - `mods/tmux/v1/cli`
  - `mods/shared/nixplan`
  - `mods/shell/v1/cli`
- direct daemon verification:
  - `./dialtone_mod dialtone v1 ensure`
  - `./dialtone_mod dialtone v1 status --full`
- visible acceptance:
  - `./dialtone_mod test v1 start --codex-wait-seconds 90 --wait-seconds 60`
- full visible sweep:
  - `./dialtone_mod shell v1 test-all --wait-seconds 240`
  - completed with `DIALTONE_TEST_ALL_DONE`

When in doubt, simplify toward:

- one shared `default` Nix shell
- `shell v1` at the center
- SQLite as the source of truth
- visible Go tests in `dialtone-view`
