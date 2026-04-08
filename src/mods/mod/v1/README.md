# Mods Mod (`v1`)

The `mods` mod is the SQLite control surface for the Dialtone mods system. It
syncs the repo into the local state database, prints dependency/state data, and
lets callers inspect the canonical `command_runs` ledger, linked transport
rows, and protocol/test records directly from SQLite.

## Quick Start

```sh
./dialtone_mod mods v1 help
./dialtone_mod mods v1 db sync
./dialtone_mod mods v1 db graph --format outline
./dialtone_mod mods v1 db runs --limit 10
./dialtone_mod mods v1 db test-plan
```

## Workflows

### Refresh The SQLite Registry

```sh
./dialtone_mod mods v1 db path
./dialtone_mod mods v1 db init
./dialtone_mod mods v1 db sync
./dialtone_mod mods v1 db graph --format outline
./dialtone_mod mods v1 db topo
```

### Inspect Queue, State, And Protocol Data

```sh
./dialtone_mod mods v1 db env
./dialtone_mod mods v1 db state
./dialtone_mod mods v1 db queue --limit 20
./dialtone_mod mods v1 db runs --limit 20
./dialtone_mod mods v1 db run --id <run_id>
./dialtone_mod mods v1 db protocol-runs --limit 10
./dialtone_mod mods v1 db protocol-events --run <run_id>
```

### Use The Shared Test Config

```sh
export DIALTONE_ENV_FILE=env/test.dialtone.json
./dialtone_mod mods v1 db sync
./dialtone_mod mods v1 db runs --limit 10
./dialtone_mod mods v1 db test-run --name default
```

### Query The SQLite Database Directly

```sh
nix --extra-experimental-features 'nix-command flakes' develop .#default \
  --command sqlite3 "$(./dialtone_mod mods v1 db path)" \
  "select id,mod_name,mod_version,verb,status,target,log_path from command_runs order by id desc limit 5;"
```

### Execute The SQLite Test Plan

```sh
./dialtone_mod mods v1 db sync
./dialtone_mod mods v1 db test-plan
./dialtone_mod mods v1 db test-run --name default
./dialtone_mod mods v1 db test-runs --limit 10
./dialtone_mod mods v1 db test-run-steps --run <run_id>
```

### Run The Go Tests For This Mod Under Nix

```sh
cd /Users/user/dialtone
DIALTONE_ENV_FILE=env/test.dialtone.json nix --extra-experimental-features 'nix-command flakes' develop .#default \
  --command zsh -lc 'cd src && go test ./mods/mod/v1 ./mods/mod/v1/cli'
```

### Windows: Use The Visible Tmux Workflow

```powershell
.\dialtone_mod.ps1 mod v1 help
.\dialtone_mod.ps1 mod v1 list
.\dialtone_mod.ps1 mod v1 db sync
.\dialtone_mod.ps1 mod v1 db graph --format outline
.\dialtone_mod.ps1 read
```

## Dependencies

- `tmux v1`
- local SQLite database at `~/.dialtone/state.sqlite`
- Nix-backed Go toolchain for build/test/sync operations
- `shell v1` for the visible Ghostty + tmux + protocol smoke-test workflow

## Test Results

- Timestamp: 2026-03-22
- Command:

```sh
./dialtone_mod shell v1 test-all --wait-seconds 240
```

- Visible result:

```text
ok  	dialtone/dev/mods/mod/v1
ok  	dialtone/dev/mods/mod/v1/cli
```
