# Tmux Mod (`v1`)

The `tmux` mod provides lightweight, remote tmux log retrieval for Dialtone hosts.
It is designed to read scrollback from a running tmux session on a remote node
over SSH (via the transport used by `dialtone_mod`).

## Command API

```bash
./dialtone_mod tmux v1 logs [--host <name|ip>] [--user <user>] [--port <port>] [--session <tmux-session>] [--pane <window.pane>] [--lines <n>] [--dry-run]
```

## `logs` command

Captures tmux scrollback from a remote host and prints it to stdout.

### Arguments

- `--host` (required): Mesh host alias or IP.
  - Defaults to `$DIALTONE_HOSTNAME`.
- `--user`: SSH user (defaults to host entry in `env/mesh.json`, then `$USER`).
- `--port`: SSH port (defaults to host entry in `env/mesh.json`, then `22`).
- `--session`: Target tmux session name.
  - Default: `dialtone-<host>`
- `--pane`: Tmux target pane (`<window>.<pane>`), default `0.0`.
- `--lines`: Number of lines to capture, default `10`.
- `--dry-run`: Print generated command without running it.

### Notes

- The mod reads host metadata from `env/mesh.json` using the same host matching
  logic as other Dialtone mods.
- If the target session is unavailable, it attempts to fall back to the first
  available tmux session on that host.
- If the requested pane fails, it falls back to the first available pane in the
  session.
- If tmux is not on `$PATH`, it tries to discover it via running process path,
  `/nix/store/*-tmux-*/bin/tmux`, and `$HOME/.nix-profile/bin/tmux`.

## Example

```bash
# Read the last 10 lines from the gold host's default tmux session
./dialtone_mod tmux v1 logs --host gold

# Read 40 lines from a specific session/pane on gold
./dialtone_mod tmux v1 logs --host gold --session dialtone-gold --pane 0.0 --lines 40

# Inspect command generated remotely without executing
./dialtone_mod tmux v1 logs --host gold --dry-run
```
