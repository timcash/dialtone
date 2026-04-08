# Mods System (`src/mods`)

This directory contains the versioned Dialtone mods system.

Use this file as the main system guide for:

- how real mods are laid out
- which CLI wrappers exist and what contract they must expose
- how `dialtone_mod`, `src/mods.go`, `dialtone v1`, and `shell v1` cooperate
- how LLM agents should inspect, test, debug, and extend the system

This README is the single source of truth for mods architecture, operator workflow, and debugging. There is no separate transition plan file to consult.

## What Counts As A Mod

Real user-facing mods live at:

```text
src/mods/<mod-name>/<version>/
```

Examples:

- `src/mods/dialtone/v1`
- `src/mods/shell/v1`
- `src/mods/codex/v1`
- `src/mods/db/v1`
- `src/mods/mesh/v3`

Not everything under `src/mods` is a real mod version:

- `src/mods/shared/*` contains shared helper packages
- `src/mods/bin/mods` is not a real mod version

Current real mod versions in this repo:

- `chrome/v1`
- `codex/v1`
- `db/v1`
- `dialtone/v1`
- `ghostty/v1`
- `mesh/v3`
- `mod/v1`
- `mosh/v1`
- `shell/v1`
- `ssh/v1`
- `test/v1`
- `tmux/v1`
- `tsnet/v1`

## Mod CLI Contract

Every real mod version should expose a Go CLI wrapper at:

```text
src/mods/<mod-name>/<version>/cli/main.go
```

That wrapper is the contract between the mod and [src/mods.go](/Users/user/dialtone/src/mods.go).

Required commands for every real mod:

- `install`
- `build`
- `format`
- `test`

Rules:

- `install|build|format|test` are the shared command names; behavior can vary by mod
- `build` outputs should land under `bin/mods/<mod>/<version>/`
- `format` should default to the mod’s own tree, not the whole repo
- `test` should be mod-local and should usually cover both the version-root package and the CLI package
- runtime/admin commands such as `start`, `status`, `queue`, `service`, `tab`, `read`, or `run` come after the basic contract

Practical meaning for agents:

- prefer `./dialtone_mod <mod> <version> install|format|test|build` over ad hoc `gofmt`, `go test`, or `go build`
- if a wrapper already exists, use it first
- if a routed mod command returns a `command_id` and `run_id`, inspect those SQLite-backed records instead of assuming the command already finished

### CLI Link-Up Pattern

`src/mods.go` does not need a hand-written case for most mods. The normal link-up path is:

1. add `src/mods/<mod-name>/<version>/mod.json`
2. add `src/mods/<mod-name>/<version>/cli/main.go`
3. make sure `cli/` is a real Go package

When that exists, [src/mods.go](/Users/user/dialtone/src/mods.go) resolves the mod to the `cli` package and runs it with `go run` from `src/`.

Keep the Go wrapper minimal. Its job is to parse subcommands, call shared helpers, and delegate to the real toolchain/runtime. Prefer these helpers from `dialtone/dev/internal/modcli`:

- `FindRepoRoot()` for repo discovery
- `ModDir()` and `SrcRoot()` for stable paths
- `BuildOutputPath()` for `bin/mods/<mod>/<version>/...`
- `GoBuildCommand()` and `GoTestCommand()` for Go-based build/test commands
- `NixDevelopCommand()` for non-Go toolchains such as `zig`, `cargo`, `bash`, or `gofmt`
- `CollectGoFiles()` and `NormalizeOptionalPathArg()` for wrapper-local formatting flows

Expected style:

- use the repo flake `default` shell unless the mod truly needs a different shell
- let Nix provide the toolchain instead of custom per-wrapper bootstrap logic
- keep environment setup in the shared helpers, not duplicated in every mod
- let the Go wrapper stay small even when the real implementation is Zig, Rust, shell, or C

### Windows Entry Point

On Windows, use `dialtone_mod.ps1` as the public mods launcher. It wraps `dialtone.ps1 tmux ...` and types `./dialtone_mod ...` into the visible WSL tmux session `dialtone`, so you can see the exact mod command and the pane output that follows.

Common Windows examples:

```powershell
.\dialtone_mod.ps1 db v1 test
.\dialtone_mod.ps1 db v1 run --benchmark
.\dialtone_mod.ps1 mod v1 list
.\dialtone_mod.ps1 status
.\dialtone_mod.ps1 read
```

## Common Workflows

### Windows: Visible Mod Workflow

```powershell
.\dialtone_mod.ps1 status
.\dialtone_mod.ps1 mod v1 help
.\dialtone_mod.ps1 mod v1 list
.\dialtone_mod.ps1 db v1 test
.\dialtone_mod.ps1 read
```

### WSL Or Linux: Inspect And Test One Mod

```sh
./dialtone_mod ssh v1 help
./dialtone_mod ssh v1 install
./dialtone_mod ssh v1 format
./dialtone_mod ssh v1 test
./dialtone_mod ssh v1 build
```

### SQLite Control Surface For Mods

```sh
./dialtone_mod mods v1 db path
./dialtone_mod mods v1 db sync
./dialtone_mod mods v1 db graph --format outline
./dialtone_mod mods v1 db runs --limit 10
./dialtone_mod mods v1 db run --id <run_id>
./dialtone_mod mods v1 db topo
./dialtone_mod mods v1 db queue --limit 20
./dialtone_mod mods v1 db protocol-runs --limit 10
```

### Shared Test Config

```sh
export DIALTONE_ENV_FILE=env/test.dialtone.json
./dialtone_mod mods v1 db sync
./dialtone_mod mods v1 db runs --limit 10
./dialtone_mod db v1 test
```

### Inspect A Routed Command

```sh
./dialtone_mod db v1 test
./dialtone_mod mods v1 db runs --limit 10
./dialtone_mod mods v1 db run --id <run_id>
./dialtone_mod dialtone v1 commands --limit 10
./dialtone_mod dialtone v1 command --row-id <command_id> --full
./dialtone_mod dialtone v1 log --kind command --row-id <command_id>
```

### Add A New Mod Version

```sh
mkdir -p src/mods/example/v1/cli
$EDITOR src/mods/example/v1/mod.json
$EDITOR src/mods/example/v1/cli/main.go

./dialtone_mod mods v1 db sync
./dialtone_mod mods v1 db graph --format outline
./dialtone_mod example v1 help
./dialtone_mod example v1 test
```

## Current Architecture

The current system has five important layers.

### 1. `dialtone_mod`

The user-facing shell wrapper stays stable and small.

Responsibilities:

- locate the repo root
- bootstrap the state directory under `~/.dialtone`
- ensure the standalone `dialtone` daemon binary exists
- decide whether a command is direct or routed
- run direct commands through [src/mods.go](/Users/user/dialtone/src/mods.go)
- send routed commands to `dialtone v1 queue`

### 2. `src/mods.go`

The main Go dispatcher is the mod router.

Responsibilities:

- normalize `<mod> <version> <command>`
- map aliases such as `mods -> mod`
- decide direct vs routed execution
- resolve the mod CLI wrapper
- launch the wrapper through Go in the repo `src/` tree

Normal execution targets the CLI wrapper for every real mod version.

### 3. Direct Control-Plane Mods

These commands execute immediately from the caller side:

- `./dialtone_mod dialtone v1 ...`
- `./dialtone_mod shell v1 ...`
- `./dialtone_mod tmux v1 ...`
- `./dialtone_mod ghostty v1 ...`
- `./dialtone_mod codex v1 ...`
- `./dialtone_mod test v1 ...`

These mods own the local control plane itself:

- `dialtone v1`: daemon/runtime inspection and queue access
- `shell v1`: visible workflow owner
- `tmux v1`: pane/session control
- `ghostty v1`: terminal window/tab/split control
- `codex v1`: prompt-pane launch/status
- `test v1`: end-to-end system harness

### 4. Routed Mods

Most other plain `./dialtone_mod <mod> <version> ...` commands are routed through SQLite and executed visibly in `dialtone-view`.

Examples:

- `./dialtone_mod mod v1 ...`
- `./dialtone_mod chrome v1 ...`
- `./dialtone_mod db v1 ...`
- `./dialtone_mod mesh v3 ...`
- `./dialtone_mod mosh v1 ...`
- `./dialtone_mod ssh v1 ...`
- `./dialtone_mod tsnet v1 ...`

Current classification:

- direct control-plane mods: `dialtone/v1`, `shell/v1`, `tmux/v1`, `ghostty/v1`, `codex/v1`, `test/v1`
- routed mods: `mod/v1`, `chrome/v1`, `db/v1`, `mesh/v3`, `mosh/v1`, `ssh/v1`, `tsnet/v1`

Expected behavior for routed commands:

- the caller usually gets a fast route report
- the report includes a `command_id` and a `run_id`
- the canonical ledger row lives in SQLite `command_runs`
- the linked `shell_bus` row is the delivery/transport row
- the actual visible execution happens in `dialtone-view`

### 5. Background `dialtone` Control Plane

The background system is one pipeline:

1. `dialtone_mod` receives a command
2. direct commands run immediately; routed commands go to `dialtone v1 queue`
3. `dialtone` records durable state in SQLite, especially `command_runs`, `shell_bus`, and state rows
4. `shell v1 serve` runs inside `dialtone-view` and executes queued visible work
5. prompt rows target `codex-view`
6. command rows target `dialtone-view`
7. `test v1 start` ties prompt submission, routed command rows, pane reads, and protocol events together

SQLite is the source of truth when stdout and pane text disagree.

### SQLite-First Model

Use this mental model when you extend the system:

- `command_runs` is the canonical record for routed mod execution
- `shell_bus` is the transport queue and pane-observation layer linked by `command_run_id`
- `state_values` and `runtime_env` keep durable control-plane settings and captured env
- `protocol_runs`, `protocol_events`, `mod_test_runs`, and `mod_test_run_steps` hold history and verification
- tmux panes and PIDs are runtime details recorded into SQLite, not the primary identity of a command

## Control Points

These files are the main control points when you need to debug or extend the mods system:

- [dialtone_mod](/Users/user/dialtone/dialtone_mod)
  Stable user-facing shell wrapper, Nix/bootstrap path, and daemon handoff
- [src/mods.go](/Users/user/dialtone/src/mods.go)
  Main Go dispatcher for direct-vs-routed execution and CLI wrapper resolution
- [modstate.go](/Users/user/dialtone/src/internal/modstate/modstate.go)
  SQLite-backed mod registry, entrypoint resolution, canonical command runs, transport rows, protocol rows, and test-run state
- `src/mods/<mod>/<version>/cli/main.go`
  Per-mod contract entrypoint for `install|build|format|test` plus runtime/admin commands
- `src/mods/<mod>/<version>/main.go`
  Standalone runtime entrypoint only when a real background/runtime binary is needed

## Direct Vs Routed Commands

This distinction matters more than anything else when you are debugging or automating the system.

Direct commands:

- execute immediately
- are used for workflow control, daemon inspection, pane control, and tests
- include `codex v1`

Routed commands:

- usually return a route report instead of final command output
- should not run in the caller terminal
- should not run in `codex-view`
- should execute visibly in `dialtone-view`

There is one important exception:

- when the worker is already executing a queued row in `dialtone-view`, the direct-execution guard allows nested `./dialtone_mod ...` calls that match that running row to execute locally instead of recursively re-queueing

If you are unsure whether a command finished or only queued, inspect it with:

```sh
./dialtone_mod mods v1 db run --id <run_id>
./dialtone_mod dialtone v1 command --row-id <command_id> --full
./dialtone_mod dialtone v1 log --kind command --row-id <command_id>
```

## Workflow Ownership

Keep this ownership split intact:

- `shell v1` owns visible workflow readiness
- `dialtone v1` supervises and reports
- `test v1 start` proves the whole prompt/worker pipeline end to end
- `codex v1` only owns prompt-pane launch/status, not the overall workflow

Practical consequences:

- `shell v1 ensure-worker` is the main readiness/recovery path
- `shell v1 start`, `run`, `prompt`, `enqueue-command`, `test`, and `test-all` should rely on shell readiness
- `dialtone v1` should expose state and process inspection, not a competing workflow bootstrap algorithm

## Which CLI To Use

Use these command families deliberately.

### `dialtone v1`

Use for daemon and SQLite inspection:

```sh
./dialtone_mod dialtone v1 paths
./dialtone_mod dialtone v1 status --full
./dialtone_mod dialtone v1 processes
./dialtone_mod dialtone v1 commands --limit 20
./dialtone_mod dialtone v1 command --row-id <id> --full
./dialtone_mod dialtone v1 log --kind command --row-id <id>
./dialtone_mod dialtone v1 protocol-runs --limit 10
./dialtone_mod dialtone v1 protocol-run --run <id> --full
./dialtone_mod dialtone v1 test-runs --limit 10
./dialtone_mod dialtone v1 test-run --run <id> --full
./dialtone_mod dialtone v1 test
```

Use this before raw SQL.

`dialtone v1 test` should stay independent of `tmux`, `ghostty`, `codex`, and the visible shell workflow. It is the right wrapper for validating:

- SQLite schema/state behavior
- `command_runs` lifecycle and linked `shell_bus` transport metadata
- daemon/process inspection formatting
- log-path resolution and protocol/test-run inspection helpers

If you need to prove the visible prompt/worker pipeline, that is a different layer:

- `./dialtone_mod shell v1 test`
- `./dialtone_mod test v1 start`

### `shell v1`

Use for visible workflow control:

```sh
./dialtone_mod shell v1 test-basic
./dialtone_mod shell v1 start --run-tests=false
./dialtone_mod shell v1 state --full
./dialtone_mod shell v1 read --role prompt --full
./dialtone_mod shell v1 read --role command --full
./dialtone_mod shell v1 test
./dialtone_mod shell v1 test-all
```

Use `shell v1 run` when you need one visible command executed in `dialtone-view` and you want the worker to own it.

### `tmux v1`

Use for explicit pane/session control and reads:

```sh
./dialtone_mod tmux v1 list
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 80
./dialtone_mod tmux v1 clear --pane codex-view:0:1
./dialtone_mod tmux v1 write --pane codex-view:0:0 --enter "hello"
./dialtone_mod tmux v1 split --pane codex-view:0:0 --direction right --title dialtone-view
```

### `ghostty v1`

Use for the macOS terminal surface around the tmux workflow:

```sh
./dialtone_mod ghostty v1 list
./dialtone_mod ghostty v1 fresh-window --cwd /Users/user/dialtone
./dialtone_mod ghostty v1 write --terminal 1 --focus "tmux new-session -A -s codex-view"
```

### `codex v1`

Use for prompt-pane launch/status and mod-local testing:

```sh
./dialtone_mod codex v1 install
./dialtone_mod codex v1 format
./dialtone_mod codex v1 test
./dialtone_mod codex v1 start --session codex-view --pane codex-view:0:0
./dialtone_mod codex v1 status --session codex-view
```

`codex v1` is direct. It is not routed through `dialtone-view`.

### `test v1`

Use for the end-to-end system harness:

```sh
./dialtone_mod test v1 format
./dialtone_mod test v1 test
./dialtone_mod test v1 start
```

`test v1 start` should remain the high-confidence proof that:

- prompt submission reaches `codex-view`
- Codex can queue one routed command from the prompt pane
- that command executes in `dialtone-view`
- SQLite tracks the whole protocol

## Recommended Agent Workflow

When working on a real mod:

1. read this file and the target mod README
2. run the mod wrapper first:
   `./dialtone_mod <mod> <version> install`
   `./dialtone_mod <mod> <version> format`
   `./dialtone_mod <mod> <version> test`
3. if the change touches the workflow or control plane, also run:
   `./dialtone_mod shell v1 test`
   `./dialtone_mod test v1 start`
4. if a mod command is routed, inspect the `command_id` instead of assuming synchronous completion
5. use `dialtone v1` inspection commands before writing SQL

## Agent Start Checklist

Before changing the mods system:

1. read this README and the target mod README
2. inspect the real mod tree if needed:
   `find src/mods -mindepth 2 -maxdepth 2 -type d | sort`
3. prefer the mod wrapper first:
   `./dialtone_mod <mod> <version> install`
   `./dialtone_mod <mod> <version> format`
   `./dialtone_mod <mod> <version> test`
4. if you touch the control plane, baseline these too:
   `./dialtone_mod shell v1 test-basic`
   `./dialtone_mod shell v1 test`
   `./dialtone_mod test v1 start`
5. add or tighten tests before broad behavioral refactors
6. update this README and the target mod README when operator workflow changes materially

When changing the control plane itself:

1. `./dialtone_mod shell v1 test-basic`
2. `./dialtone_mod shell v1 test`
3. `./dialtone_mod test v1 start`
4. inspect protocol rows and command logs if behavior looks wrong

## Recommended Debug Order

When the system looks wrong, use this order:

```sh
./dialtone_mod dialtone v1 status --full
./dialtone_mod dialtone v1 processes
./dialtone_mod dialtone v1 commands --limit 20
./dialtone_mod dialtone v1 command --row-id <id> --full
./dialtone_mod dialtone v1 log --kind command --row-id <id>
./dialtone_mod shell v1 read --role prompt --full
./dialtone_mod shell v1 read --role command --full
./dialtone_mod dialtone v1 protocol-runs --limit 10
./dialtone_mod dialtone v1 protocol-run --run <id> --full
```

Only drop to `sqlite3` or `./dialtone_mod mods v1 db ...` when:

- you are adding a new DB feature
- the inspection surface is missing what you need
- you are debugging a schema/query bug

## Healthy Vs Unhealthy State

Healthy control-plane signs:

- `./dialtone_mod dialtone v1 status --full` shows non-`missing` `prompt_target` and `command_target`
- `worker_status` is `running`
- `ensure_running` stays low
- `./dialtone_mod dialtone v1 processes` shows one daemon and one worker pair, not a growing fan-out of `ensure-worker` wrappers

Unhealthy signs that usually mean workflow supervision is looping:

- `prompt_target` or `command_target` is `missing`
- `ensure_running` keeps increasing
- `dialtone v1 processes` shows many repeated `dialtone_mod shell v1 ensure-worker --wait-seconds 30` processes
- `shell v1 test-basic` or `test v1 start` stalls before a new protocol run row appears

If the state is unhealthy, inspect the daemon and latest command first:

```sh
./dialtone_mod dialtone v1 status --full
./dialtone_mod dialtone v1 processes
./dialtone_mod dialtone v1 log --kind daemon --lines 80
./dialtone_mod dialtone v1 commands --limit 10
```

## Current Agent Rules

- use wrapper commands first
- do not assume a routed command finished just because it returned
- do not add custom environment prefixes in front of plain `./dialtone_mod ...` commands unless the task truly requires it
- restart the workflow or worker after changing `dialtone v1`, `shell v1`, worker logging, pane routing, or prompt submission logic
- keep the user-facing command shape plain and stable
- shared helper packages are not real mods and do not need their own user-facing wrapper

## Guardrails

- do not change the plain `./dialtone_mod ...` command shape
- do not move queue ownership out of `dialtone v1`
- do not let routed commands execute in the caller terminal or in `codex-view`
- do not duplicate visible-workflow readiness logic across `dialtone v1`, `shell v1`, and `test v1`
- keep readiness in `shell v1` and keep daemon/process/SQLite inspection in `dialtone v1`
- do not bypass wrapper commands with host `go build`, `gofmt`, or `go test` when the mod wrapper already exists
- keep `format` defaulting to the target mod tree instead of drifting into repo-wide sweeps
- keep build outputs under `bin/mods/<mod>/<version>/`

## Fast Reference

```sh
# control-plane preflight
./dialtone_mod shell v1 test-basic

# visible workflow
./dialtone_mod shell v1 start --run-tests=false

# mod-local contract workflow
./dialtone_mod <mod> <version> install
./dialtone_mod <mod> <version> format
./dialtone_mod <mod> <version> test
./dialtone_mod <mod> <version> build

# broader workflow regression
./dialtone_mod shell v1 test
./dialtone_mod test v1 start

# routed-command inspection
./dialtone_mod mods v1 db run --id <id>
./dialtone_mod dialtone v1 command --row-id <id> --full
./dialtone_mod dialtone v1 log --kind command --row-id <id>
```
