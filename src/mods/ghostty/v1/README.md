# Ghostty Mod (`v1`)

The `ghostty` mod provides local Ghostty automation on macOS through the
native AppleScript API.

It targets the selected tab of the front Ghostty window and can address
individual terminals in that tab directly, which makes it suitable for
choosing the correct Ghostty split before tmux or Codex take over.

## Command API

```bash
./dialtone_mod ghostty v1 list
./dialtone_mod ghostty v1 write [--terminal 1] [--enter=true|false] [--focus=false] <text...>
```

## Commands

### `list`

Lists the terminals in the selected tab of the front Ghostty window.

Output includes:

- 1-based terminal index
- Ghostty terminal id
- terminal name
- working directory
- whether the terminal is currently focused

### `write`

Writes text into a specific terminal in the selected tab of the front
Ghostty window.

- `--terminal`: 1-based terminal index in the selected tab
- `--enter`: send Enter after typing text (default `true`)
- `--focus`: focus the target terminal before typing (default `false`)

## Typical Use

The Ghostty mod is the pane selector for the local workflow.

Typical flow:

1. Inspect the front Ghostty tab with `ghostty v1 list`.
2. Send `tmux new-session -A -s codex-view` into the second pane.
3. Use `codex v1 start --session codex-view` to launch Codex in that tmux session.
4. Use `tmux v1 read/write` for later interaction.

## Examples

```bash
# Show the terminals in the selected tab of the front Ghostty window
./dialtone_mod ghostty v1 list

# Start or attach codex-view in the second Ghostty pane
./dialtone_mod ghostty v1 write --terminal 2 "tmux new-session -A -s codex-view"

# Type into the second split pane without moving focus
./dialtone_mod ghostty v1 write --terminal 2 "echo hello from ghostty v1"

# Type and focus that pane first
./dialtone_mod ghostty v1 write --terminal 2 --focus "pwd"
```
