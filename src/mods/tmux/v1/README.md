# Tmux Mod (`v1`)

`tmux v1` is the low-level tmux control surface for Dialtone.

Code layout:

```text
src/mods/tmux/v1/
├── README.md
├── mod.json
├── nix.packages
├── main_test.go
└── cli/
    ├── main.go
    └── main_test.go
```

`tmux v1` is intentionally lower level than `shell v1`.
Use `shell v1` for normal local workflow orchestration and use `tmux v1` when you need direct pane/session control.

## Quick Start

```sh
# List sessions.
./dialtone_mod tmux v1 list

# Read a pane directly.
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 20

# Clear a pane and its history.
./dialtone_mod tmux v1 clear --pane codex-view:0:1

# Split a pane.
./dialtone_mod tmux v1 split --pane codex-view:0:0 --direction right --title dialtone-view --cwd /Users/user/dialtone

# Enter the repo's default nix shell in the right pane.
./dialtone_mod tmux v1 shell --pane codex-view:0:1 --shell default

# Run the standardized recursive Go tests.
./dialtone_mod shell v1 run --wait-seconds 60 \
  "clear && cd /Users/user/dialtone/src && go test ./mods/tmux/v1/..."
```

## DIALTONE>

```text
$ ./dialtone_mod tmux v1 clear --pane codex-view:0:1
cleared tmux pane codex-view:0.1

$ ./dialtone_mod tmux v1 shell --pane codex-view:0:1 --shell default
entered nix shell default in codex-view:0.1
```

## Dependencies

- tmux
- Nix
- `shell v1` as the preferred high-level wrapper

## Test Results

- Timestamp: 2026-03-22
- Commands:

```sh
./dialtone_mod shell v1 run --wait-seconds 180 \
  "clear && cd /Users/user/dialtone/src && go test ./mods/tmux/v1/cli ./mods/shared/nixplan ./mods/shell/v1/cli"

./dialtone_mod shell v1 test-all --wait-seconds 240
```

- Visible result:

```text
ok  	dialtone/dev/mods/tmux/v1/cli
ok  	dialtone/dev/mods/tmux/v1
```
