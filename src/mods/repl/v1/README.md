# REPL v1

Minimal local REPL mod scaffold.

Run it through the repo CLI:

```bash
./dialtone_mod repl v1 run
./dialtone_mod repl v1 run --once "hello"
./dialtone_mod repl v1 logs --tail 20
```

Or enter the focused flake shell first:

```bash
nix develop .#repl-v1
./dialtone_mod repl v1 test
```

Files stay local to this mod:

- runtime logs: `src/mods/repl/v1/runtime/repl.log`
- tests: `src/mods/repl/v1/*_test.go`
- Nix-backed operational commands: `src/mods/repl/v1/cli`

## Quick Start

```sh
# Show the repl command surface from the sqlite-managed mods system.
./dialtone_mod repl v1 help

# Run the Go test package for this mod from the repo source tree.
cd /Users/user/dialtone/src
go test ./mods/repl/v1

# Regenerate the sqlite DAG and the stepwise test plan before the next TDD loop.
cd ..
./dialtone_mod mods v1 db sync
./dialtone_mod mods v1 db test-plan
```

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
# step 6 -> repl v1 (passed)
go test ./mods/repl/v1

```

Observed result summary:

- step 6 `repl v1` -> `passed` (exit=0)
