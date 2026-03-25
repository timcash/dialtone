# Shell Mod (`v1`)

`shell v1` is the primary local control plane for the Ghostty + tmux + Codex workflow.

Plain `./dialtone_mod ...` commands should stay plain. The intended flow is:

- `dialtone_mod` bootstraps the Nix-backed toolchain on first run
- `dialtone_mod` starts the background `dialtone` process when needed
- agent terminal queues the command in SQLite
- `dialtone_mod` returns immediately with a `command_id`, queue state, and a control-plane process report
- the long-lived `shell v1 serve` worker in `dialtone-view` prints and runs it
- SQLite stores the command status, PID, exit code, runtime, and output

Code layout:

```text
src/mods/shell/v1/
├── README.md
├── mod.json
├── nix.packages
├── main_test.go
└── cli/
    ├── main.go
    └── main_test.go
```

## Quick Start

```sh
# Run the core preflight tests first.
./dialtone_mod shell v1 test-basic

# Start the preferred local workflow:
# - one Ghostty window
# - one tab
# - left pane: codex-view
# - right pane: dialtone-view running the SQLite shell worker
./dialtone_mod shell v1 start --run-tests=false

# Run the full mod test sweep visibly in dialtone-view.
./dialtone_mod shell v1 test-all

# Or run the full sequence in one command:
# - test-basic
# - start
# - test-all
./dialtone_mod shell v1 workflow

# Read the left pane through SQLite-backed shell state.
./dialtone_mod shell v1 read --role prompt

# Read the right pane through SQLite-backed shell state.
./dialtone_mod shell v1 read --role command

# Check worker status, queued work, and the latest routed command result.
./dialtone_mod shell v1 status --full

# Use dialtone v1 for focused control-plane inspection.
./dialtone_mod dialtone v1 commands --limit 10
./dialtone_mod dialtone v1 command --row-id 901 --full
./dialtone_mod dialtone v1 log --kind command --row-id 901

# Plain mod commands are routed through SQLite into dialtone-view.
./dialtone_mod mods v1 db graph --format outline

# Run the shell workflow Go suite visibly in dialtone-view.
./dialtone_mod shell v1 test

# Run a one-off visible command in dialtone-view.
./dialtone_mod shell v1 run --wait-seconds 60 \
  "clear && cd /Users/user/dialtone/src && go test ./mods/shell/v1/..."

# Inspect shell bus state and recent events from SQLite.
./dialtone_mod shell v1 state --full
./dialtone_mod shell v1 events --limit 10

# Run the full end-to-end system harness.
./dialtone_mod test v1 start
```

## Readiness Model

`shell v1` now owns visible-workflow readiness.

That means:

- `shell v1 ensure-worker` is the single setup/recovery path
- `shell v1 run`, `shell v1 prompt`, `shell v1 enqueue-command`, `shell v1 test`, and `shell v1 test-all` should all rely on that preflight path before enqueueing visible work
- the readiness check verifies:
  - `prompt_target`
  - `command_target`
  - tmux pane reachability for both targets
  - worker heartbeat / running state
  - worker pane matching `command_target`
- if targets or panes are missing, the shell workflow should be recreated
- if panes exist but the worker is stale, only the worker should be restarted
- if preflight still cannot produce a healthy workflow, the command should fail with concrete reasons instead of hanging

Current known live blocker:

- the Go preflight tests are green
- the live `shell v1 start` / `shell v1 test` path can still loop queued `codex v1 start` rows after the panes are created
- treat that as the next shell-workflow bug to fix before trusting the visible startup path as fully healthy

## DIALTONE>

```text
$ ./dialtone_mod shell v1 test-basic
ok  	dialtone/dev/internal/modstate	...
ok  	dialtone/dev/mods/shared/sqlitestate	...
ok  	dialtone/dev/mods/mod/v1	...
ok  	dialtone/dev/mods/shell/v1/cli	...

$ ./dialtone_mod shell v1 start --run-tests=false
created fresh ghostty window ...
started shell workflow: ghostty one-window/one-tab -> codex-view:0:0 (codex) + codex-view:0:1 (dialtone-view) -> codex gpt-5.4

$ ./dialtone_mod mods v1 db graph --format outline
route	queued
command_id	71
inspect	./dialtone_mod dialtone v1 command --row-id 71 --full

$ ./dialtone_mod shell v1 test
ran command via shell bus [row_id=72]

$ ./dialtone_mod shell v1 test-all
ran command via shell bus [row_id=72]

$ ./dialtone_mod shell v1 read --pane codex-view:0:1 --full
role	command
pane	codex-view:0:1
text
user@gold src % clear && cd /Users/user/dialtone/src && go test ./mods/... && printf 'DIALTONE_TEST_ALL_DONE\n'
ok  	dialtone/dev/mods/chrome/v1	...
ok  	dialtone/dev/mods/codex/v1	...
ok  	dialtone/dev/mods/ghostty/v1	...
ok  	dialtone/dev/mods/shell/v1	...
ok  	dialtone/dev/mods/tmux/v1	...
ok  	dialtone/dev/mods/tsnet/v1/cli	...
DIALTONE_TEST_ALL_DONE
```

## Dependencies

- `ghostty v1`
- `tmux v1`
- `codex v1`
- macOS
- tmux
- Ghostty
- Nix

## Test Results

- Timestamp: 2026-03-23
- Commands:

```sh
env DIALTONE_NIX_SHELL_BANNER=0 nix --extra-experimental-features 'nix-command flakes' --no-warn-dirty develop .#default --command \
  bash -lc 'cd /Users/user/dialtone/src && go test ./mods/shell/v1/cli ./mods/dialtone/v1 ./mods/ghostty/v1/cli ./mods/tmux/v1/cli'

./dialtone_mod shell v1 test
```

- Visible result:

```text
ok  	dialtone/dev/mods/shell/v1/cli
ok  	dialtone/dev/mods/dialtone/v1
ok  	dialtone/dev/mods/ghostty/v1/cli
ok  	dialtone/dev/mods/tmux/v1/cli

live note:
./dialtone_mod shell v1 test now recreates the workflow instead of silently trusting stale worker state,
but the visible startup path still loops queued ./dialtone_mod codex v1 start rows after pane creation.
```
