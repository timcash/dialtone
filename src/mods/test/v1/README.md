# Test Mod (`v1`)

`test v1` is the end-to-end system harness for the dialtone client, daemon, SQLite control plane, and the `codex-view` / `dialtone-view` tmux panes.

`./dialtone_mod test v1 start` is intentionally a client-side orchestration command:

- it runs in the Nix-backed shell, not in `dialtone-view`
- it ensures the visible workflow exists
- it restarts `codex-view:0:0` into a fresh Codex prompt pane through the shell worker
- it waits for the Codex CLI banner before submitting the prompt
- it submits a prompt to `codex-view`
- it waits for one new SQLite `shell_bus` command row that Codex itself creates after the prompt
- it queues a deterministic matrix of plain routed `./dialtone_mod ...` commands
- it validates success, long-running, failure, invalid-input, recovery, and background behavior in SQLite
- it proves the routed commands executed in `dialtone-view`, not `codex-view`
- it prints the SQLite-backed system status, both pane reads, an architecture summary, and protocol events

Current acceptance boundary:

- it proves prompt delivery to `codex-view`
- it proves Codex itself can run one plain routed `./dialtone_mod ...` command from `codex-view`
- it proves that Codex-initiated row is queued in SQLite after the prompt row and then executed in `dialtone-view`
- it proves routed command execution in `dialtone-view`
- it proves the worker survives long-running, failure, invalid-input, recovery, and background cases
- it still uses the harness to queue the larger scenario matrix after the first Codex-initiated command

## Quick Start

```sh
./dialtone_mod test v1 start
```

Expected behavior:

- `dialtone_mod` ensures `dialtone` and the shell worker are running
- `dialtone-view` refreshes the left prompt pane into Codex before the test prompt is sent
- `test v1 start` waits for the Codex banner in `codex-view` before it submits the prompt
- `codex-view` receives a visible prompt that asks Codex to run exactly one routed probe command
- Codex queues one plain routed command first:
  - `./dialtone_mod mods v1 probe --mode success --label CODEX_AGENT_<token>`
- `dialtone-view` then runs the remaining deterministic scenario matrix of plain routed commands:
  - `./dialtone_mod mods v1 db graph --format outline`
  - `./dialtone_mod mods v1 probe --mode sleep --sleep-ms 1500 --label TEST_LONG_RUNNING`
  - `./dialtone_mod mods v1 probe --mode fail --label TEST_FAILURE`
  - `./dialtone_mod mods v1 probe --mode invalid --label TEST_INVALID_MODE`
  - `./dialtone_mod mods v1 probe --mode success --label TEST_RECOVERY`
  - `./dialtone_mod mods v1 probe --mode background --sleep-ms 4000 --label TEST_BACKGROUND --background-file <marker-file>`
- SQLite records the protocol run, shell bus rows, pane snapshots, and system state
- the architecture summary shows:
  - `worker_matches_command_target=true`
  - `prompt_visible_in_codex_view=true`
  - `prompt_not_visible_in_dialtone_view=true`
  - `codex_initiated_command=true`
  - `codex_command_ran_in_dialtone_view=true`
  - `recovery_after_failures=true`
  - `background_completion=true`
- the final stdout report includes:
  - `protocol_run_id`
  - `prompt_row_id`
  - `scenario_count`
  - one summary line per scenario
  - `command_row_id`
  - `command_exit_code`
  - `command_runtime_ms`
  - the cached `shell v1 status`
  - the latest prompt and command pane text

After a run, the intended inspection path is:

```sh
./dialtone_mod dialtone v1 command --row-id <command_row_id> --full
./dialtone_mod dialtone v1 log --kind command --row-id <command_row_id>
./dialtone_mod dialtone v1 protocol-run --run <protocol_run_id> --full
```

## Next Steps

The next acceptance upgrade for `test v1 start` is to expand the agent-driven part of the proof:

- prompt Codex to run more than one routed command with distinct expected outcomes
- let Codex drive at least one failure case and one background case instead of only the first success probe
- keep waiting for SQLite `shell_bus` rows created after the prompt instead of trusting chat output
- prove those rows still target `dialtone-view`
- record enough metadata in SQLite to distinguish harness-queued rows from Codex-initiated rows
- keep the deterministic scenario matrix as the fallback control-plane regression suite

## Test Results

- Timestamp: 2026-03-23
- Command:

```sh
./dialtone_mod test v1 start
```

- Visible result:

```text
[test.ensure-worker]
shell worker already running

[test.architecture]
codex_initiated_command	true
codex_command_row_id	1574
codex_command_ran_in_dialtone_view	true
worker_matches_command_target	true
prompt_visible_in_codex_view	true
prompt_not_visible_in_dialtone_view	true
routed_command_count	7
failure_scenarios	failing,invalid_mode
recovery_after_failures	true
background_completion	true

test_result	passed
protocol_run_id	18
prompt_row_id	1571
scenario_count	7
command_row_id	1597
command_exit_code	0
command_runtime_ms	382
```
