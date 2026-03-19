# Tmux Mod (`v1`)

The `tmux` mod provides local tmux session utilities for Dialtone.

It is local-only, talks to the default tmux server, and targets panes with a
single `session:window:pane` selector.

## Command API

```bash
./dialtone_mod tmux v1 list [--short]
./dialtone_mod tmux v1 write [--pane codex-view:1:1] [--enter] <text...>
./dialtone_mod tmux v1 read [--pane codex-view:1:1] [--lines 10]
./dialtone_mod tmux v1 rename [--session NAME] [--to dialtone]
./dialtone_mod tmux v1 shell [--pane codex-view:1:1] [--shell default|repl-v1|ssh-v1]
./dialtone_mod tmux v1 target [--set codex-view:1:1] [--clear]
```

## Pane Target Format

Commands that operate on panes use:

- `--pane <session:window:pane>`

Examples:

- `codex-view:1:1`
- `gold-codex:0:1`
- `ops:1:2`

Use native tmux to discover the current pane ids when needed:

```bash
tmux list-panes -a -F '#{session_name}:#{window_index}:#{pane_index}'
```

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

### `shell`

Puts the target pane into a Dialtone repo Nix shell without starting Codex.

- `--pane`: target pane in `session:window:pane` format.
- `--shell`: flake shell name (`default`, `repl-v1`, `ssh-v1`).

### `target`

Persists or clears the default tmux pane that `./dialtone_mod` should proxy
non-control commands into.

- `--set`: save a `session:window:pane` target for later `dialtone_mod` proxying
- `--clear`: remove the saved target

When a target is set, normal non-control commands like `./dialtone_mod repl v1 test`
are sent into that tmux pane instead of running directly in the caller shell.

## Typical Use

The tmux mod is the control surface once the session already exists.

Typical flow:

1. Use `ghostty v1` to target the correct Ghostty split.
2. Start or attach a normal tmux session there.
3. Optionally use `tmux v1 shell` to put the pane into the repo Nix shell.
4. Optionally use `tmux v1 target --set ...` to make `dialtone_mod` proxy into that pane.
5. Use `codex v1` to launch Codex inside that tmux session.
6. Use `tmux v1` to read or write commands to the live pane.

## Examples

```bash
# List sessions
./dialtone_mod tmux v1 list
./dialtone_mod tmux v1 list --short

# Read the active codex-view pane
./dialtone_mod tmux v1 read --pane codex-view:1:1 --lines 40

# Put codex-view into the repo default nix shell without starting Codex
./dialtone_mod tmux v1 shell --pane codex-view:1:1 --shell default

# Make dialtone_mod send non-control commands into codex-view
./dialtone_mod tmux v1 target --set codex-view:1:1

# Show the currently configured proxy target
./dialtone_mod tmux v1 target

# Send a shell command into codex-view
./dialtone_mod tmux v1 write --pane codex-view:1:1 --enter "pwd"

# Ask Codex something through the same tmux pane
./dialtone_mod tmux v1 write --pane codex-view:1:1 --enter "summarize the current git diff"

# Rename a session
./dialtone_mod tmux v1 rename --session codex-view --to dialtone

# Clear proxy mode
./dialtone_mod tmux v1 target --clear
```
