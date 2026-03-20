# Tmux Mod (`v1`)

The `tmux` mod provides local tmux session utilities for Dialtone.

It is local-only, talks to the default tmux server, and targets panes with a
single `session:window:pane` selector.

In practice, the safest way to discover a pane selector is still native tmux:

```sh
# Show the canonical tmux pane selectors from the default tmux server.
tmux list-panes -a -F '#{session_name}:#{window_index}:#{pane_index}'
```

## Command API

```bash
./dialtone_mod tmux v1 list [--short]
./dialtone_mod tmux v1 write [--pane codex-view:0:0] [--enter] <text...>
./dialtone_mod tmux v1 read [--pane codex-view:0:0] [--lines 10]
./dialtone_mod tmux v1 clear [--pane codex-view:0:0]
./dialtone_mod tmux v1 rename [--session NAME] [--to dialtone]
./dialtone_mod tmux v1 shell [--pane codex-view:0:0] [--shell default|repl-v1|ssh-v1]
./dialtone_mod tmux v1 target [--set codex-view:0:0] [--clear]
```

## Pane Target Format

Commands that operate on panes use:

- `--pane <session:window:pane>`

Examples:

- `codex-view:0:0`
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

### `clear`

Clears the target pane visually and drops its tmux scrollback history.

- `--pane`: target pane in `session:window:pane` format.

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

## Known-Good Flow

Use this when you have already selected the correct Ghostty tab.

```sh
# Attach to an existing session or create it in the selected Ghostty tab.
./dialtone_mod ghostty v1 write --terminal 1 "tmux new-session -A -s codex-view"

# Verify the actual pane selector using tmux itself.
tmux list-panes -a -F '#{session_name}:#{window_index}:#{pane_index}'

# Read the current prompt from the tmux pane.
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 20

# Clear the pane before the next step.
./dialtone_mod tmux v1 clear --pane codex-view:0:0

# Send a command into the pane and press Enter.
./dialtone_mod tmux v1 write --pane codex-view:0:0 --enter "pwd"

# Verify tmux is responding inside the target pane.
./dialtone_mod tmux v1 write --pane codex-view:0:0 --enter "tmux display-message -p '#S:#I:#P'"
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 30
```

## Fresh Session Reset

Use this when you want to remove the existing tmux session and start over.

```sh
# Inspect existing tmux sessions before deleting anything.
tmux list-sessions -F '#{session_name}' 2>/dev/null || true

# Kill the old session if it exists.
tmux kill-session -t codex-view 2>/dev/null || true

# Confirm the session is gone.
tmux list-sessions -F '#{session_name}' 2>/dev/null || true

# Recreate it by sending the attach-or-create command through Ghostty.
./dialtone_mod ghostty v1 write --terminal 1 "tmux new-session -A -s codex-view"
```

## Troubleshooting

```sh
# If `tmux v1 list` output looks surprising, compare it with tmux's own view.
./dialtone_mod tmux v1 list
tmux list-panes -a -F '#{session_name}:#{window_index}:#{pane_index}'

# If tmux says it cannot connect to the default socket, there is no tmux server
# yet. Start one from Ghostty first.
./dialtone_mod ghostty v1 write --terminal 1 "tmux new-session -A -s codex-view"

# If a write/read command fails with "can't find window", double-check that the
# pane selector really exists and use the value printed by native tmux.
tmux list-panes -a -F '#{session_name}:#{window_index}:#{pane_index}'
```

## Examples

```bash
# List sessions
./dialtone_mod tmux v1 list
./dialtone_mod tmux v1 list --short

# Read the active codex-view pane
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 40

# Clear the dialtone-view pane
./dialtone_mod tmux v1 clear --pane codex-view:0:1

# Put codex-view into the repo default nix shell without starting Codex
./dialtone_mod tmux v1 shell --pane codex-view:0:0 --shell default

# Make dialtone_mod send non-control commands into codex-view
./dialtone_mod tmux v1 target --set codex-view:0:0

# Show the currently configured proxy target
./dialtone_mod tmux v1 target

# Send a shell command into codex-view
./dialtone_mod tmux v1 write --pane codex-view:0:0 --enter "pwd"

# Ask Codex something through the same tmux pane
./dialtone_mod tmux v1 write --pane codex-view:0:0 --enter "summarize the current git diff"

# Rename a session
./dialtone_mod tmux v1 rename --session codex-view --to dialtone

# Clear proxy mode
./dialtone_mod tmux v1 target --clear
```
### `split`

Splits an existing tmux pane and returns the new pane target.

- `--pane`: source pane in `session:window:pane` form
- `--direction`: `right|left|down|up`
- `--title`: optional title for the new pane
- `--cwd`: optional working directory for the new pane
- `--command`: optional initial command for the new pane
- `--focus`: focus the new pane after splitting (default `true`)

Example:

```sh
# Split codex-view:0:0 to the right and title the new pane dialtone-view.
./dialtone_mod tmux v1 split --pane codex-view:0:0 --direction right --title dialtone-view --cwd /Users/user/dialtone
```
