# Shell Mod (`v1`)

The `shell` mod turns the local Ghostty + tmux + Codex bootstrap into one
command.

It is intentionally opinionated:

- quit Ghostty first
- create exactly one fresh Ghostty window
- keep exactly one tab in that window
- start or attach `codex-view`
- set the tmux proxy target to `codex-view:0:0`
- launch Codex CLI in that session
- optionally split the tmux window so Codex stays left and `dialtone-view`
  appears right

## Quick Start

```sh
# Start the full local interactive workflow in one command.
# This quits Ghostty, creates one fresh window with one tab, starts or attaches
# `codex-view`, sets the tmux target, and launches Codex CLI there.
./dialtone_mod shell v1 start

# Start the same workflow but choose a different Codex model.
./dialtone_mod shell v1 start --model gpt-5.4

# Start the workflow but use a different flake shell before Codex launches.
./dialtone_mod shell v1 start --shell repl-v1

# Wait longer for the tmux pane if local startup is slow.
./dialtone_mod shell v1 start --wait-seconds 30

# Inspect the single selected Ghostty terminal after startup.
# You should see exactly one terminal in the selected tab.
./dialtone_mod ghostty v1 list

# Read the live tmux pane after startup.
# This is how you verify that the startup banner reached Codex itself.
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 40

# Split the tmux window vertically.
# Codex stays on the left in `codex-view:0:0`, and a nix shell pane titled
# `dialtone-view` is created on the right in `codex-view:0:1`.
./dialtone_mod shell v1 split-vertical

# Run the SQLite-backed protocol smoke test against the current visible panes.
# This records prompt/command events in SQLite and expects a visible
# `./dialtone_mod` command to succeed in `dialtone-view`.
./dialtone_mod shell v1 demo-protocol --bootstrap=false

# Inspect the SQLite-backed tmux target and queued proxy commands after startup.
./dialtone_mod mods v1 db state --key tmux.target
./dialtone_mod mods v1 db queue --name tmux --limit 10

# Inspect the latest protocol run directly from the controller shell.
env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db protocol-runs --limit 5
env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db protocol-events --run 2
```

## DIALTONE>

```text
$ ./dialtone_mod shell v1 start
created fresh ghostty window (id=tab-group-8ae4ade00) tab (id=tab-8aee84800) terminal (id=E15ED691-F19A-4D16-982A-D42D0A483754)
wrote to ghostty terminal 1 (id=E15ED691-F19A-4D16-982A-D42D0A483754): tmux new-session -A -s codex-view
set dialtone_mod tmux target: codex-view:0:0
DIALTONE> sent to tmux target codex-view:0:0: codex v1 start --session codex-view --shell default --reasoning medium --model gpt-5.4
started shell workflow: ghostty one-window/one-tab -> codex-view:0:0 -> codex gpt-5.4

$ ./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 12
Starting Codex CLI with gpt-5.4 (requested reasoning: medium) and skipping confirmations...
╭────────────────────────────────────────────╮
│ >_ OpenAI Codex (v0.115.0)                 │
│                                            │
│ model:     gpt-5.4 high   /model to change │
│ directory: ~/dialtone                      │
╰────────────────────────────────────────────╯

$ ./dialtone_mod shell v1 split-vertical
split codex-view:0.0 right -> codex-view:0.1
entered nix shell default in codex-view:0.1
cleared tmux pane codex-view:0.1
set dialtone_mod tmux target: codex-view:0:0
split shell workflow: codex on codex-view:0:0 (left), dialtone-view on codex-view:0:1 (right)

$ ./dialtone_mod shell v1 demo-protocol --bootstrap=false
demo protocol run 2 passed
prompt_target	codex-view:0:0
command_target	codex-view:0:1
command	env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db graph --format outline
result	observed "- shell:v1" in codex-view:0:1
```

Expected behavior after the command returns:

- Ghostty is open with one fresh window and one tab
- that tab is attached to the `codex-view` tmux session
- Codex CLI is starting or already running in that pane
- the startup path suppresses the Codex self-update chooser so the workflow
  does not stop on an interactive update prompt
- prompts should be submitted into `codex-view:0:0`
- visible `./dialtone_mod` commands should run in `codex-view:0:1`
- `split-vertical` keeps `codex-view:0:0` on the left and makes
  `dialtone-view` the right-hand tmux pane at `codex-view:0:1`
- `split-vertical` clears `dialtone-view` after entering the Nix shell so the
  right pane lands on a clean prompt
- `demo-protocol` records a `protocol_runs` row and ordered `protocol_events`
  rows in SQLite while proving that one visible `./dialtone_mod` command can be
  observed successfully in `dialtone-view`

## Dependencies

Runtime command dependencies:

- `ghostty v1`
- `tmux v1`
- `codex v1`

Runtime environment dependencies:

- macOS
- Ghostty
- tmux
- Nix

## Test Results

Most recent validation run:

- `<timestamp-start>`: 2026-03-20T23:59:00Z
- `<timestamp-stop>`: 2026-03-21T00:00:00Z
- `<runtime>`: ~60s across the final formatting/test/demo loop
- `<ERRORS>`: none in the accepted `demo-protocol --bootstrap=false` run
- `<ui-screenshot-grid>`: not captured; verification was performed by reading the live `codex-view:0:0` and `codex-view:0:1` tmux panes plus direct SQLite queries

Most recent command set:

```text
cd /Users/user/dialtone && nix --extra-experimental-features 'nix-command flakes' develop .#default --command zsh -lc 'cd src && gofmt -w ./mods/shell/v1/main.go && go test ./mods/shell/v1'
./dialtone_mod shell v1 demo-protocol --bootstrap=false
env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 40
env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod tmux v1 read --pane codex-view:0:1 --lines 120
nix --extra-experimental-features 'nix-command flakes' develop .#default --command sqlite3 .dialtone/state.sqlite "select id,name,status,prompt_target,command_target,result_text from protocol_runs order by id desc limit 5;"
```

Observed result summary:

- `go test ./mods/shell/v1` passed under Nix in the visible `dialtone-view` pane
- `demo protocol run 2 passed`
- the recorded visible command was `env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db graph --format outline`
- `protocol_runs` recorded `passed` with prompt target `codex-view:0:0` and command target `codex-view:0:1`
- `protocol_events` recorded `workflow_ready`, `prompt_submitted`, `command_written`, and `command_observed`
```
