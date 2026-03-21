# Ghostty Mod (`v1`)

The `ghostty` mod provides local Ghostty automation on macOS through Ghostty's
native AppleScript API.

It is the local UI control surface for the Ghostty + tmux + Codex workflow:

- create a new window or tab
- split a terminal inside the selected tab
- focus a terminal in the selected tab
- type text into a terminal in the selected tab
- inspect the terminals currently visible in the selected tab

`ghostty v1` always targets the selected tab of the front Ghostty window.

## Quick Start

```sh
# Show the ghostty command surface from the sqlite-managed mods system.
./dialtone_mod ghostty v1 help

# Run the Go test package for this mod from the repo source tree.
cd /Users/user/dialtone/src
go test ./mods/ghostty/v1

# Regenerate the sqlite DAG and the stepwise test plan before the next TDD loop.
cd ..
./dialtone_mod mods v1 db sync
./dialtone_mod mods v1 db test-plan
```

## DIALTONE>

```text
$ ./dialtone_mod ghostty v1 new-tab --cwd /Users/user/dialtone
created ghostty tab 2 (id=tab-c5c374a00) terminal (id=FC1DB632-5FFC-484E-B9C9-BDF9410E8359)

$ ./dialtone_mod ghostty v1 list
1	focused=true	id=FC1DB632-5FFC-484E-B9C9-BDF9410E8359	name=~/dialtone	cwd=/Users/user/dialtone

$ ./dialtone_mod ghostty v1 split --terminal 1 --direction right
split ghostty terminal 1 right -> terminal (id=7BEBD1BB-9CD1-4FEE-8D3E-6B4FEDBB068F)

$ ./dialtone_mod ghostty v1 list
1	focused=false	id=FC1DB632-5FFC-484E-B9C9-BDF9410E8359	name=~/dialtone	cwd=/Users/user/dialtone
2	focused=true	id=7BEBD1BB-9CD1-4FEE-8D3E-6B4FEDBB068F	name=~/dialtone	cwd=/Users/user/dialtone

$ ./dialtone_mod ghostty v1 write --terminal 2 "tmux new-session -A -s codex-view"
wrote to ghostty terminal 2 (id=7BEBD1BB-9CD1-4FEE-8D3E-6B4FEDBB068F): tmux new-session -A -s codex-view

$ tmux list-panes -a -F '#{session_name}:#{window_index}:#{pane_index}'
codex-view:0:0
```

This is the interaction pattern future agents should expect:

- `new-tab` and `new-window` print the created object ids
- `split` prints the new terminal id
- `focus` prints the focused terminal id
- `fullscreen` prints the front window id and target fullscreen state
- `write` echoes the terminal index, terminal id, and submitted text
- `list` prints one line per terminal in the selected tab

## Dependencies

Hard mod dependencies:

- none

Common workflow companions:

- `tmux v1`
- `codex v1`

Runtime environment dependencies:

- macOS
- Ghostty with AppleScript support
- `/usr/bin/osascript`

## Test Results

Most recent validation run:

- `<run-id>`: 2
- `<plan-name>`: default
- `<timestamp-start>`: 2026-03-20T23:26:51Z
- `<timestamp-stop>`: 2026-03-20T23:26:58Z
- `<runtime>`: 7s
- `<status>`: failed
- `<ERRORS>`: none
- `<ui-screenshot-grid>`: not captured

Most recent command set:

```sh
# step 2 -> ghostty v1 (passed)
go test ./mods/ghostty/v1

```

Observed result summary:

- step 2 `ghostty v1` -> `passed` (exit=0)
