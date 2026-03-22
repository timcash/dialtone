# Codex Mod (`v1`)

`codex v1` launches and inspects the tmux-backed Codex session used by the local Dialtone workflow.

Code layout:

```text
src/mods/codex/v1/
├── README.md
├── mod.json
├── main_test.go
└── cli/
    ├── main.go
    └── main_test.go
```

`codex v1` should stay focused on:

- starting Codex in the selected tmux session/pane
- reporting Codex pane/session status

Normal day-to-day orchestration should happen through `shell v1`.

## Quick Start

```sh
# Start the full local shell workflow.
./dialtone_mod shell v1 start --run-tests=false

# Launch or relaunch Codex in codex-view.
./dialtone_mod codex v1 start --session codex-view

# Show the session status.
./dialtone_mod codex v1 status --session codex-view

# Run the standardized recursive Go tests.
./dialtone_mod shell v1 run --wait-seconds 60 \
  "clear && cd /Users/user/dialtone/src && go test ./mods/codex/v1/..."
```

## DIALTONE>

```text
$ ./dialtone_mod codex v1 start --session codex-view
Starting Codex CLI with gpt-5.4 ...

$ ./dialtone_mod codex v1 status --session codex-view
codex-view	0.0	go	/Users/user/dialtone
```

## Dependencies

- `tmux v1`
- `shell v1`
- Codex CLI available through the repo Nix shell

## Test Results

- Timestamp: 2026-03-22
- Command:

```sh
./dialtone_mod shell v1 test-all --wait-seconds 240
```

- Visible result:

```text
ok  	dialtone/dev/mods/codex/v1
ok  	dialtone/dev/mods/codex/v1/cli
```
