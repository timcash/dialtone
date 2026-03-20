# Codex Mod (`v1`)

The `codex` mod controls a local tmux-backed Codex session on the default tmux
server.

It is intended for the Ghostty + tmux workflow where Ghostty selects the pane,
tmux owns the long-running `codex-view` session, and `codex v1` moves that
session into the Dialtone repo and launches Codex through the repo Nix shell.

## Command API

```bash
./dialtone_mod codex v1 start [--session codex-view] [--shell default|repl-v1|ssh-v1] [--reasoning medium] [--model gpt-5.4]
./dialtone_mod codex v1 status [--session codex-view]
```

## Commands

### `start`

Ensures the tmux session exists on the default tmux server, resolves its active
pane, and sends the startup command that:

- `cd`s into the Dialtone repo
- enters the requested flake shell with `nix develop`
- launches Codex with `codex` or `npx --yes @openai/codex`
- forces `CI=1` and `-c check_for_update_on_startup=false` so the Codex
  self-update chooser does not block the automated startup flow

Flags:

- `--session`: tmux session name (default `codex-view`)
- `--shell`: flake shell to enter before launching Codex
- `--reasoning`: startup label for the banner text
- `--model`: Codex model to launch

### `status`

Shows the live tmux pane state for the selected Codex session:

- session name
- pane id
- current command
- current working directory

## Typical Use

The Codex mod is the launcher for the repo-aware Codex session.

Typical flow:

1. Use `ghostty v1` to target the desired Ghostty pane.
2. Send `tmux new-session -A -s codex-view` into that pane.
3. Run `./dialtone_mod codex v1 start --session codex-view`.
4. Confirm with `./dialtone_mod codex v1 status --session codex-view`.
5. Use `tmux v1 write/read` to interact with the session after startup.

## End-to-End Bootstrap

Use this full sequence when you want a reproducible local launch path from
Ghostty through tmux into Codex.

```sh
# Start from a clean Ghostty state with one window and one tab rooted at the repo.
# Keep raw AppleScript inside the Ghostty mod; use the mod CLI here.
./dialtone_mod ghostty v1 fresh-window --cwd /Users/user/dialtone

# Confirm the selected tab has a terminal to target.
./dialtone_mod ghostty v1 list

# Start or attach the tmux session in that tab.
./dialtone_mod ghostty v1 write --terminal 1 "tmux new-session -A -s codex-view"

# Persist the canonical pane target used by the tested one-window/one-tab flow.
./dialtone_mod tmux v1 target --set codex-view:0:0

# Launch Codex inside the tmux session using the repo's default flake shell.
./dialtone_mod codex v1 start --session codex-view

# Confirm the live pane state after startup.
./dialtone_mod codex v1 status --session codex-view
```

## One-Command Bootstrap

Use `shell v1` when you want the preferred fully automated path:

```sh
# Reset Ghostty to one window and one tab, attach codex-view, set the tmux
# proxy target, and launch Codex in one command.
./dialtone_mod shell v1 start
```

## Troubleshooting

```sh
# If Codex startup seems stuck, first read the live pane output. In the tested
# one-window/one-tab workflow, the canonical pane is `codex-view:0:0`.
./dialtone_mod tmux v1 target --set codex-view:0:0

# Read recent output from the live pane to see whether the shell, nix develop,
# or Codex process is waiting for input.
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 60

# Check the current command and cwd for the managed Codex session.
./dialtone_mod codex v1 status --session codex-view
```

## Examples

```bash
# Start Codex in codex-view using the repo default nix shell
./dialtone_mod codex v1 start --session codex-view

# Start Codex in a focused repl shell
./dialtone_mod codex v1 start --session codex-view --shell repl-v1

# Start Codex with a different model
./dialtone_mod codex v1 start --session codex-view --model gpt-5.4

# Show the pane command and cwd for the session
./dialtone_mod codex v1 status --session codex-view
```
