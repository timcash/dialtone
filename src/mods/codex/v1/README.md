# Codex Mod (`v1`)

The `codex` mod controls a local tmux-backed Codex session on the default tmux
server.

It is intended for the Ghostty + tmux workflow where Ghostty selects the pane
and tmux owns the long-running `codex-view` session.

## Command API

```bash
./dialtone_mod codex v1 start [--session codex-view] [--shell default|repl-v1|ssh-v1] [--reasoning medium] [--model gpt-5.4]
./dialtone_mod codex v1 status [--session codex-view]
```

## Examples

```bash
# Start Codex in the default tmux session using the repo default nix shell
./dialtone_mod codex v1 start

# Start Codex in a focused repl shell
./dialtone_mod codex v1 start --shell repl-v1

# Show the pane command and cwd for the session
./dialtone_mod codex v1 status
```
