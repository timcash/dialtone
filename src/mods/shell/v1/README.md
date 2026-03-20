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
```

Expected behavior after the command returns:

- Ghostty is open with one fresh window and one tab
- that tab is attached to the `codex-view` tmux session
- Codex CLI is starting or already running in that pane
- the startup path suppresses the Codex self-update chooser so the workflow
  does not stop on an interactive update prompt
- future non-control `./dialtone_mod` commands are proxied into `codex-view:0:0`
- `split-vertical` keeps `codex-view:0:0` on the left and makes
  `dialtone-view` the right-hand tmux pane at `codex-view:0:1`
- `split-vertical` clears `dialtone-view` after entering the Nix shell so the
  right pane lands on a clean prompt

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

- `<timestamp-start>`: 2026-03-20T18:58:30Z
- `<timestamp-stop>`: 2026-03-20T19:02:52Z
- `<runtime>`: 4m22s
- `<ERRORS>`: none in the final accepted run; earlier validation exposed that Codex still showed its self-update chooser until `codex v1` forced `CI=1`
- `<ui-screenshot-grid>`: not captured; verification was performed by reading the live `codex-view:0:0` tmux pane and confirming Ghostty had one terminal in the selected tab

Most recent command set:

```text
./dialtone_mod tmux v1 shell --pane codex-view:0:0 --shell default
./dialtone_mod tmux v1 write --pane codex-view:0:0 --enter "cd /Users/user/dialtone/src && gofmt -w mods/codex/v1/main.go mods/codex/v1/main_test.go mods/ghostty/v1/main.go mods/ghostty/v1/main_test.go mods/shell/v1/main.go mods/shell/v1/main_test.go && go vet ./mods/codex/v1 ./mods/ghostty/v1 ./mods/shell/v1 && go test ./mods/codex/v1 ./mods/ghostty/v1 ./mods/shell/v1 && go build -o /tmp/dialtone-codex-v1 ./mods/codex/v1 && go build -o /tmp/dialtone-ghostty-v1 ./mods/ghostty/v1 && go build -o /tmp/dialtone-shell-v1 ./mods/shell/v1"
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 80
./dialtone_mod shell v1 start
./dialtone_mod ghostty v1 list
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 40

Observed result summary:
- `go vet ./mods/codex/v1 ./mods/ghostty/v1 ./mods/shell/v1` passed
- `go test ./mods/codex/v1 ./mods/ghostty/v1 ./mods/shell/v1` passed
- `/tmp/dialtone-codex-v1`, `/tmp/dialtone-ghostty-v1`, and `/tmp/dialtone-shell-v1` were produced
- `./dialtone_mod shell v1 start` recreated Ghostty as one window with one tab
- the Ghostty terminal visibly received `tmux new-session -A -s codex-view`
- `./dialtone_mod ghostty v1 list` showed one focused terminal in the selected tab
- the final proxied `codex v1 start` reached the Codex TUI without the update chooser appearing
```
