# Test Mod (`v1`)

`test v1` is the end-to-end system harness for the dialtone client, daemon, SQLite control plane, and the `codex-view` / `dialtone-view` tmux panes.

`./dialtone_mod test v1 start` is intentionally a client-side orchestration command:

- it runs in the Nix-backed shell, not in `dialtone-view`
- it ensures the visible workflow exists
- it submits a prompt to `codex-view`
- it queues one plain routed `./dialtone_mod ...` command
- it waits for the SQLite shell bus row to finish
- it prints the SQLite-backed system status, both pane reads, and protocol events

## Quick Start

```sh
./dialtone_mod test v1 start
```

Expected behavior:

- `dialtone_mod` ensures `dialtone` and the shell worker are running
- `codex-view` receives a visible prompt
- `dialtone-view` runs `./dialtone_mod mods v1 db graph --format outline`
- SQLite records the protocol run, shell bus rows, pane snapshots, and system state
- the final stdout report includes:
  - `protocol_run_id`
  - `prompt_row_id`
  - `command_row_id`
  - `command_exit_code`
  - `command_runtime_ms`
  - the cached `shell v1 status`
  - the latest prompt and command pane text

## Test Results

- Timestamp: 2026-03-23
- Command:

```sh
./dialtone_mod test v1 start
```

- Expected result:

```text
test_result	passed
protocol_run_id	...
prompt_row_id	...
command_row_id	...
command_exit_code	0
command_runtime_ms	...
```
