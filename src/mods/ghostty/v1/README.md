# Ghostty Mod (`v1`)

`ghostty v1` is the macOS UI automation layer for the local Dialtone workflow.

Code layout:

```text
src/mods/ghostty/v1/
├── README.md
├── mod.json
├── main_test.go
└── cli/
    ├── main.go
    └── main_test.go
```

`ghostty v1` should stay focused on Ghostty itself:

- windows
- tabs
- selected-tab terminals
- splits
- focus
- fullscreen
- text input

## Quick Start

```sh
# Show the command surface.
./dialtone_mod ghostty v1 help

# Create a fresh window at the repo root.
./dialtone_mod ghostty v1 fresh-window --cwd /Users/user/dialtone

# Inspect the selected tab's terminals.
./dialtone_mod ghostty v1 list

# Split the selected terminal.
./dialtone_mod ghostty v1 split --terminal 1 --direction right

# Run the Go tests for this standardized mod layout.
./dialtone_mod shell v1 run --wait-seconds 60 \
  "clear && cd /Users/user/dialtone/src && go test ./mods/ghostty/v1/..."
```

## DIALTONE>

```text
$ ./dialtone_mod ghostty v1 fresh-window --cwd /Users/user/dialtone
created fresh ghostty window ...

$ ./dialtone_mod ghostty v1 list
1	focused=true	...

$ ./dialtone_mod ghostty v1 split --terminal 1 --direction right
split ghostty terminal 1 right -> terminal (...)
```

## Dependencies

- macOS
- Ghostty
- `/usr/bin/osascript`

## Test Results

- Timestamp: 2026-03-22
- Command:

```sh
./dialtone_mod shell v1 test-all --wait-seconds 240
```

- Visible result:

```text
ok  	dialtone/dev/mods/ghostty/v1
ok  	dialtone/dev/mods/ghostty/v1/cli
```
