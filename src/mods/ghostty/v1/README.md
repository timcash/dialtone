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
# Show the terminals in the selected tab of the front Ghostty window.
./dialtone_mod ghostty v1 list

# Quit Ghostty, then recreate exactly one fresh window with one tab.
# Use this when you need a predictable starting point for automation.
./dialtone_mod ghostty v1 fresh-window --cwd /Users/user/dialtone

# Create and select a new tab in the front Ghostty window.
# Use --cwd when you want the new tab to start inside the Dialtone repo.
./dialtone_mod ghostty v1 new-tab --cwd /Users/user/dialtone

# Create and activate a new Ghostty window.
# Use --command if you want the initial terminal to run something immediately.
./dialtone_mod ghostty v1 new-window --cwd /Users/user/dialtone --command "tmux new-session -A -s codex-view"

# Split terminal 1 in the selected tab into left/right panes.
# `right` and `left` create a vertical divider.
./dialtone_mod ghostty v1 split --terminal 1 --direction right

# Split terminal 1 into top/bottom panes.
# `down` and `up` create a horizontal divider.
./dialtone_mod ghostty v1 split --terminal 1 --direction down

# Focus terminal 2 in the selected tab.
./dialtone_mod ghostty v1 focus --terminal 2

# Make the front Ghostty window fullscreen.
./dialtone_mod ghostty v1 fullscreen

# Exit fullscreen on the front Ghostty window.
./dialtone_mod ghostty v1 fullscreen --on=false

# Type into terminal 2 and press Enter.
./dialtone_mod ghostty v1 write --terminal 2 "tmux new-session -A -s codex-view"

# Type into terminal 1 without pressing Enter yet.
./dialtone_mod ghostty v1 write --terminal 1 --enter=false "echo queued but not submitted"

# Focus terminal 1 first, then type and press Enter.
./dialtone_mod ghostty v1 write --terminal 1 --focus "pwd"

# Create a tab that opens in the repo and immediately sends initial input.
# This is useful when you want a shell plus a first typed command.
./dialtone_mod ghostty v1 new-tab --cwd /Users/user/dialtone --input "echo ready"

# Typical Dialtone bootstrap:
# 1. reset Ghostty to one window and one tab
# 2. inspect terminal indices
# 3. start or attach tmux in terminal 1
./dialtone_mod ghostty v1 fresh-window --cwd /Users/user/dialtone
./dialtone_mod ghostty v1 list
./dialtone_mod ghostty v1 write --terminal 1 "tmux new-session -A -s codex-view"
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

- `<timestamp-start>`: 2026-03-20T17:33:55Z
- `<timestamp-stop>`: 2026-03-20T17:35:14Z
- `<runtime>`: 79s
- `<ERRORS>`: none in the final Nix-backed run; an earlier long-lived shell in `codex-view` had a stale Go 1.24/1.25 toolchain mix before the repo Nix pins were updated to Go 1.25
- `<ui-screenshot-grid>`: not captured; live Ghostty smoke validation was confirmed through `./dialtone_mod ghostty v1` command output in the `codex-view` tmux pane

Most recent command set:

```text
cd /Users/user/dialtone/src
nix shell nixpkgs#go_1_25 -c bash -lc 'GO_BIN=/nix/store/hj3rkkp4azj65qvalnbl6ax0sgrfgmgh-go-1.25.7/bin/go; export GOROOT=; export GOTOOLCHAIN=local; gofmt -w mods/ghostty/v1/main.go mods/ghostty/v1/main_test.go && go test ./mods/ghostty/v1 && go build -o /tmp/dialtone-ghostty-v1 ./mods/ghostty/v1'

cd /Users/user/dialtone
./dialtone_mod ghostty v1 help
./dialtone_mod ghostty v1 new-tab --cwd /Users/user/dialtone
./dialtone_mod ghostty v1 list
./dialtone_mod ghostty v1 split --terminal 1 --direction right --focus=false
./dialtone_mod ghostty v1 list
./dialtone_mod ghostty v1 focus --terminal 1
./dialtone_mod ghostty v1 write --terminal 1 --enter=false 'echo smoke'
./dialtone_mod ghostty v1 new-window --cwd /Users/user/dialtone
./dialtone_mod ghostty v1 list

Observed result summary:
- `go test ./mods/ghostty/v1` passed
- `go build -o /tmp/dialtone-ghostty-v1 ./mods/ghostty/v1` passed
- `help`, `new-tab`, `new-window`, `split`, `focus`, `write`, and `list` all executed successfully from the `codex-view` tmux session
```
