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

- Timestamp: 2026-03-22
- Command:

```sh
./dialtone_mod shell v1 test-all --wait-seconds 240
```

- Visible result:

```text
ok  	dialtone/dev/mods/repl/v1
ok  	dialtone/dev/mods/repl/v1/cli
```
