# Shell Mod (`v1`)

`shell v1` is the primary local control plane for the Ghostty + tmux + Codex workflow.

Code layout:

```text
src/mods/shell/v1/
├── README.md
├── mod.json
├── nix.packages
├── main_test.go
└── cli/
    ├── main.go
    └── main_test.go
```

## Quick Start

```sh
# Run the core preflight tests first.
./dialtone_mod shell v1 test-basic

# Start the preferred local workflow:
# - one Ghostty window
# - one tab
# - left pane: codex-view
# - right pane: dialtone-view
./dialtone_mod shell v1 start --run-tests=false

# Run the full mod test sweep visibly in dialtone-view.
./dialtone_mod shell v1 test-all

# Or run the full sequence in one command:
# - test-basic
# - start
# - test-all
./dialtone_mod shell v1 workflow

# Read the left pane through SQLite-backed shell state.
./dialtone_mod shell v1 read --role prompt

# Read the right pane through SQLite-backed shell state.
./dialtone_mod shell v1 read --role command

# Run the shell mod's Go tests visibly in dialtone-view.
./dialtone_mod shell v1 test

# Run a one-off visible command in dialtone-view.
./dialtone_mod shell v1 run --wait-seconds 60 \
  "clear && cd /Users/user/dialtone/src && go test ./mods/shell/v1/..."

# Inspect shell bus state and recent events from SQLite.
./dialtone_mod shell v1 state --full
./dialtone_mod shell v1 events --limit 10
```

## DIALTONE>

```text
$ ./dialtone_mod shell v1 test-basic
ok  	dialtone/dev/internal/modstate	...
ok  	dialtone/dev/mods/shared/sqlitestate	...
ok  	dialtone/dev/mods/mod/v1	...
ok  	dialtone/dev/mods/shell/v1/cli	...

$ ./dialtone_mod shell v1 start --run-tests=false
created fresh ghostty window ...
started shell workflow: ghostty one-window/one-tab -> codex-view:0:0 (codex) + codex-view:0:1 (dialtone-view) -> codex gpt-5.4

$ ./dialtone_mod shell v1 test-all
ran command via shell bus [row_id=70]

$ ./dialtone_mod shell v1 read --pane codex-view:0:1 --full
role	command
pane	codex-view:0:1
text
user@gold src % clear && cd /Users/user/dialtone/src && go test ./mods/... && printf 'DIALTONE_TEST_ALL_DONE\n'
ok  	dialtone/dev/mods/chrome/v1	...
ok  	dialtone/dev/mods/codex/v1	...
ok  	dialtone/dev/mods/ghostty/v1	...
ok  	dialtone/dev/mods/shell/v1	...
ok  	dialtone/dev/mods/tmux/v1	...
ok  	dialtone/dev/mods/tsnet/v1/cli	...
DIALTONE_TEST_ALL_DONE
```

## Dependencies

- `ghostty v1`
- `tmux v1`
- `codex v1`
- macOS
- tmux
- Ghostty
- Nix

## Test Results

- Timestamp: 2026-03-22
- Commands:

```sh
./dialtone_mod shell v1 run --wait-seconds 180 \
  "clear && cd /Users/user/dialtone/src && go test ./mods/shared/router ./mods/tmux/v1/cli ./mods/shared/nixplan ./mods/shell/v1/cli"

./dialtone_mod shell v1 test-all --wait-seconds 240
```

- Visible result:

```text
ok  	dialtone/dev/mods/shared/router
ok  	dialtone/dev/mods/tmux/v1/cli
ok  	dialtone/dev/mods/shared/nixplan
ok  	dialtone/dev/mods/shell/v1/cli

ok  	dialtone/dev/mods/chrome/v1
ok  	dialtone/dev/mods/chrome/v1/cli
ok  	dialtone/dev/mods/codex/v1
ok  	dialtone/dev/mods/codex/v1/cli
ok  	dialtone/dev/mods/ghostty/v1
ok  	dialtone/dev/mods/ghostty/v1/cli
ok  	dialtone/dev/mods/mesh/v3
ok  	dialtone/dev/mods/mod/v1
ok  	dialtone/dev/mods/mod/v1/cli
ok  	dialtone/dev/mods/mosh/v1
ok  	dialtone/dev/mods/mosh/v1/cli
ok  	dialtone/dev/mods/repl/v1
ok  	dialtone/dev/mods/repl/v1/cli
ok  	dialtone/dev/mods/shared/dispatch
ok  	dialtone/dev/mods/shared/nixplan
ok  	dialtone/dev/mods/shared/router
ok  	dialtone/dev/mods/shared/sqlitestate
ok  	dialtone/dev/mods/shell/v1
ok  	dialtone/dev/mods/shell/v1/cli
ok  	dialtone/dev/mods/ssh/v1
ok  	dialtone/dev/mods/tmux/v1
ok  	dialtone/dev/mods/tmux/v1/cli
ok  	dialtone/dev/mods/tsnet/v1
ok  	dialtone/dev/mods/tsnet/v1/cli
DIALTONE_TEST_ALL_DONE
```
