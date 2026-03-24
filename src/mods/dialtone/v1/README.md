# Dialtone Mod (`v1`)

`dialtone v1` is the standalone control-plane daemon behind plain `./dialtone_mod ...` routing.

The intended flow is:

- `dialtone_mod` bootstraps the local install and builds the standalone `dialtone` binary when needed
- `dialtone` runs outside Nix and owns cached SQLite-backed control-plane state
- routed plain `./dialtone_mod ...` commands are queued in SQLite
- the long-lived `shell v1 serve` worker in `dialtone-view` runs the queued command for real
- `dialtone v1 status` reads the same SQLite state back quickly without reentering the worker

Ownership rule:

- `shell v1` owns visible-workflow readiness and pane/worker recovery
- `dialtone v1` owns background supervision, process state, log paths, and SQLite-backed inspection
- `dialtone v1` should not become a second place that tries to decide whether the visible workflow layout is healthy

Code layout:

```text
src/mods/dialtone/v1/
├── README.md
├── cli/
│   ├── main.go
│   └── main_test.go
├── mod.json
├── main.go
└── main_test.go
```

`dialtone v1` now follows the same CLI-wrapper contract as the other real mods:

- `src/mods/dialtone/v1/cli/main.go` owns `install|build|format|test`
- `src/mods/dialtone/v1/main.go` is the standalone daemon runtime
- `src/mods.go` dispatches the user-facing mod command through the CLI wrapper

The runtime is still special because `dialtone build` produces a standalone host binary that the shell wrapper can run outside the Nix shell for the long-lived background daemon.

## Quick Start

```sh
# Ensure the standalone control-plane daemon is running.
./dialtone_mod dialtone v1 ensure

# Read the cached control-plane and latest-command status directly from SQLite.
./dialtone_mod dialtone v1 status --full

# Show the path layout the control plane is using.
./dialtone_mod dialtone v1 paths

# Queue one routed plain dialtone_mod command.
./dialtone_mod mods v1 db graph --format outline

# Inspect queued commands and then one command in detail.
./dialtone_mod dialtone v1 commands --limit 10
./dialtone_mod dialtone v1 command --row-id <command_id> --full

# Read the known log file for that command or the daemon itself.
./dialtone_mod dialtone v1 log --kind command --row-id <command_id>
./dialtone_mod dialtone v1 log --kind daemon

# Inspect protocol and mod test history without SQL.
./dialtone_mod dialtone v1 protocol-runs --limit 10
./dialtone_mod dialtone v1 test-runs --limit 10

# Run the end-to-end architecture proof.
./dialtone_mod test v1 start
```

Important testing split:

- `./dialtone_mod dialtone v1 test` is the daemon/control-plane suite. It should validate SQLite state, queue rows, process inspection, and log-path behavior without requiring `tmux`, `ghostty`, `codex`, or the visible shell workflow to be live.
- `./dialtone_mod test v1 start` is the end-to-end prompt/worker proof and intentionally depends on the visible workflow.

If a legacy `dialtone_mod __dialtone serve` daemon is still present, `./dialtone_mod dialtone v1 ensure` replaces it with the standalone `dialtone` binary.

## Dependencies

- `shell v1`
- SQLite state at `~/.dialtone/state.sqlite`
- the repo-root `dialtone_mod` wrapper
- the repo-root `.tmux.conf` for the local tmux control path when present

## Test Results

- Timestamp: 2026-03-23
- Commands:

```sh
./dialtone_mod dialtone v1 format
./dialtone_mod dialtone v1 test
./dialtone_mod dialtone v1 ensure
./dialtone_mod dialtone v1 status --full
./dialtone_mod test v1 start
```

- Visible result:

```text
ok  	dialtone/dev/mods/dialtone/v1
ok  	dialtone/dev/mods/shared/dispatch

17939	/Users/user/.dialtone/logs/dialtone-daemon-1774296493315387000.log	started

state_source	sqlite_cached
prompt_target	codex-view:0:0
command_target	codex-view:0:1
dialtone_pid	17939
dialtone_status	running

command_log_path	/Users/user/.dialtone/logs/commands/shell-bus-1597.log

test_result	passed
protocol_run_id	18
prompt_row_id	1571
codex_command_row_id	1574
command_row_id	1597
codex_initiated_command	true
codex_command_ran_in_dialtone_view	true
background_completion	true
```

- Current note:
  `dialtone v1` should stay focused on daemon/process/SQLite inspection and queue ownership. Visible workflow bootstrap and pane recovery belong to `shell v1`.
