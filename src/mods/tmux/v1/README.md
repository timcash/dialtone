# Tmux Mod (`v1`)

The `tmux` mod provides local tmux session utilities for Dialtone.

It is local-only (no SSH/host routing) and targets panes with a single
`session:window:pane` selector.

## Command API

```bash
./dialtone_mod tmux v1 list [--short]
./dialtone_mod tmux v1 write [--pane dialtone:0:0] [--enter] <text...>
./dialtone_mod tmux v1 read [--pane dialtone:0:0] [--lines 10]
./dialtone_mod tmux v1 rename [--session NAME] [--to dialtone]
```

## Pane Target Format

Commands that operate on panes use:

- `--pane <session:window:pane>`

Default value:

- `dialtone:0:0`

Examples:

- `dialtone:0:0`
- `ops:1:2`

## Commands

### `list`

Lists local tmux sessions.

- `--short`: print only session names.

### `write`

Writes text into the target pane.

- `--pane`: target pane in `session:window:pane` format.
- `--enter`: additionally send Enter (`C-m`) after writing.

By default, `write` only writes text and does not press Enter.

### `read`

Reads trailing scrollback lines from the target pane.

- `--pane`: target pane in `session:window:pane` format.
- `--lines`: number of lines to read (default `10`).

### `rename`

Renames a tmux session.

- `--session`: existing session name (defaults to current session, then first).
- `--to`: new session name (default `dialtone`).

## Examples

```bash
# List sessions
./dialtone_mod tmux v1 list
./dialtone_mod tmux v1 list --short

# Write text to default target (dialtone:0:0)
./dialtone_mod tmux v1 write "echo hello"

# Write text and press Enter
./dialtone_mod tmux v1 write --enter "echo hello"

# Read last 10 lines from default target
./dialtone_mod tmux v1 read

# Read last 40 lines from a specific pane
./dialtone_mod tmux v1 read --pane dialtone:0:0 --lines 40

# Rename current session to dialtone
./dialtone_mod tmux v1 rename --to dialtone
```
