# Mods Mod (`v1`)

The `mods` mod is the SQLite control surface for the Dialtone mods system. It
syncs the repo into the local state database, prints dependency/state data, and
lets callers inspect queue/state records directly from SQLite.

## Quick Start

```sh
# Show the mod command surface from the sqlite-managed mods system.
./dialtone_mod mods v1 help

# Regenerate the sqlite DAG and the stepwise test plan before the next TDD loop.
./dialtone_mod mods v1 db sync
./dialtone_mod mods v1 db graph --format outline
./dialtone_mod mods v1 db test-plan

# Inspect the latest protocol run that ties SQLite, codex-view, and
# dialtone-view together.
env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db protocol-runs --limit 5
env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db protocol-events --run 2

# Or query the tables directly through sqlite.
nix --extra-experimental-features 'nix-command flakes' develop .#default \
  --command sqlite3 .dialtone/state.sqlite \
  "select id,name,status,prompt_target,command_target,result_text from protocol_runs order by id desc limit 5;"

# Run the Go test package for this mod under Nix.
cd /Users/user/dialtone
nix --extra-experimental-features 'nix-command flakes' develop .#default \
  --command zsh -lc 'cd src && go test ./mods/mod/v1'
```

## Dependencies

- `tmux v1`
- local SQLite database at `.dialtone/state.sqlite`
- Nix-backed Go toolchain for build/test/sync operations
- `shell v1` for the visible Ghostty + tmux + protocol smoke-test workflow

## Test Results

Most recent validation run:

- `<run-id>`: protocol run 2 plus package tests
- `<plan-name>`: default and `demo-protocol`
- `<timestamp-start>`: 2026-03-20T23:59:00Z
- `<timestamp-stop>`: 2026-03-21T00:00:00Z
- `<runtime>`: ~60s across the final Nix test and protocol verification
- `<status>`: passed for `go test ./mods/mod/v1`; passed for protocol run 2
- `<ERRORS>`: none
- `<ui-screenshot-grid>`: not captured

Most recent command set:

```sh
# package test
cd /Users/user/dialtone
nix --extra-experimental-features 'nix-command flakes' develop .#default \
  --command zsh -lc 'cd src && go test ./mods/mod/v1'

# protocol evidence
nix --extra-experimental-features 'nix-command flakes' develop .#default \
  --command sqlite3 .dialtone/state.sqlite \
  "select id,name,status,prompt_target,command_target,result_text from protocol_runs order by id desc limit 5;"

```

Observed result summary:

- `go test ./mods/mod/v1` passed under Nix
- SQLite recorded `protocol_runs.id=2` with `status=passed`
- the recorded visible command was `env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db graph --format outline`
