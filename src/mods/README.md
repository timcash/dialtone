# Mods System (`src/mods`)

This directory is the versioned mods system for Dialtone.

This file is the single system guide for:

- how to structure a mod
- how to run the visible shell workflow
- how SQLite tracks state and tests
- how an LLM should work inside the mods system

## LLM Start Here

If you are a new LLM working in `src/mods`, use this workflow first.

Goal:

- keep `shell v1` at the center
- keep SQLite as the source of truth
- run all visible tests in `dialtone-view`
- write the Go test first, then change code, then rerun the test

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

# 4. Run one focused visible test for the mod you are changing.
# Always prefer recursive package tests so both the version-root smoke test
# and the CLI tests run together.
./dialtone_mod shell v1 run --wait-seconds 120 \
  "clear && cd /Users/user/dialtone/src && go test ./mods/<mod-name>/<version>/..."

# 5. After the focused test passes, run the broader visible suite.
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
- Prefer `./dialtone_mod shell v1 run ...` for focused work.
- Prefer `./dialtone_mod shell v1 test-all` for the full visible sweep.
- Treat `ghostty v1`, `tmux v1`, and `codex v1` as backend mods behind `shell v1`.
- Read state from SQLite through the shell/mods commands when possible.

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
./dialtone_mod shell v1 events --limit 20

# inspect the DAG / test plan
./dialtone_mod mods v1 db graph --format outline
./dialtone_mod mods v1 db test-plan
```

## Core Model

The preferred local workflow is:

- one Ghostty window
- one Ghostty tab
- one tmux session: `codex-view`
- left pane: `codex-view:0:0`
- right pane: `codex-view:0:1` titled `dialtone-view`

Roles:

- `codex-view`
  Prompt and reasoning pane
- `dialtone-view`
  Visible command and test pane
- SQLite
  Source of truth for targets, queue rows, snapshots, DAG, and test state
- `shell v1`
  High-level orchestrator for the visible workflow

Treat `ghostty v1`, `tmux v1`, and `codex v1` as supporting mods behind `shell v1`.

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
- version-root `main_test.go` is a smoke/layout contract test
- CLI behavior tests live in `cli/main_test.go`
- new Go mods should not put the runnable entrypoint at the version root

## SQLite

The shared state database is:

```text
~/.dialtone/state.sqlite
```

SQLite stores:

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
  - `mods/shared/router`
  - `mods/tmux/v1/cli`
  - `mods/shared/nixplan`
  - `mods/shell/v1/cli`
- full visible sweep:
  - `./dialtone_mod shell v1 test-all --wait-seconds 240`
  - completed with `DIALTONE_TEST_ALL_DONE`

When in doubt, simplify toward:

- one shared `default` Nix shell
- `shell v1` at the center
- SQLite as the source of truth
- visible Go tests in `dialtone-view`
