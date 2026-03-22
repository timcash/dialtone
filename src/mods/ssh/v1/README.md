# Ssh Mod (`v1`)

This README is synchronized by `./dialtone_mod mods v1 db test-run`.

## Quick Start

```sh
# Show the ssh command surface from the sqlite-managed mods system.
./dialtone_mod ssh v1 help

# Run the Go test package for this mod from the repo source tree.
cd /Users/user/dialtone/src
go test ./mods/ssh/v1

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
ok  	dialtone/dev/mods/ssh/v1
```
