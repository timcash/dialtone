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
