# Ghostty Mod (`v1`)

The `ghostty` mod provides local Ghostty automation on macOS through the
native AppleScript API.

It targets the selected tab of the front Ghostty window and can address
individual terminals in that tab directly.

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

## Examples

```bash
# Show the two terminals in the current Ghostty tab
./dialtone_mod ghostty v1 list

# Type into the second split pane without moving focus
./dialtone_mod ghostty v1 write --terminal 2 "echo hello from ghostty v1"

# Type and focus that pane first
./dialtone_mod ghostty v1 write --terminal 2 --focus "pwd"
```
