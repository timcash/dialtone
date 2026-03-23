# Dialtone Mod (`v1`)

`dialtone v1` is the standalone control-plane daemon behind plain `./dialtone_mod ...` routing.

The intended flow is:

- `dialtone_mod` bootstraps the local install and builds the standalone `dialtone` binary when needed
- `dialtone` runs outside Nix and owns cached SQLite-backed control-plane state
- routed plain `./dialtone_mod ...` commands are queued in SQLite
- the long-lived `shell v1 serve` worker in `dialtone-view` runs the queued command for real
- `dialtone v1 status` reads the same SQLite state back quickly without reentering the worker

Code layout:

```text
src/mods/dialtone/v1/
├── README.md
├── mod.json
├── main.go
└── main_test.go
```

`dialtone v1` is the current exception to the usual `cli/` layout because the wrapper builds it as a standalone host binary.

## Quick Start

```sh
# Ensure the standalone control-plane daemon is running.
./dialtone_mod dialtone v1 ensure

# Read the cached control-plane and latest-command status directly from SQLite.
./dialtone_mod dialtone v1 status --full

# Queue one routed plain dialtone_mod command.
./dialtone_mod mods v1 db graph --format outline

# Inspect that queued command later by row id.
./dialtone_mod shell v1 status --row-id <command_id> --full --sync=false

# Run the end-to-end architecture proof.
./dialtone_mod test v1 start
```

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
./dialtone_mod shell v1 run --wait-seconds 300 \
  "cd /Users/user/dialtone/src && gofmt -w ./mods/dialtone/v1/main.go ./mods/dialtone/v1/main_test.go && go test ./mods/dialtone/v1 ./mods/shared/dispatch"

./dialtone_mod dialtone v1 ensure

./dialtone_mod dialtone v1 status --full

./dialtone_mod test v1 start --codex-wait-seconds 90 --wait-seconds 60
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

test_result	passed
protocol_run_id	18
prompt_row_id	1571
codex_command_row_id	1574
command_row_id	1597
codex_initiated_command	true
codex_command_ran_in_dialtone_view	true
background_completion	true
```
