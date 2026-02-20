# REPL Plugin

The REPL plugin tests interactive `dialtone.sh` behavior (`USER-1>` / `DIALTONE>` flow).

## CLI
```bash
./dialtone.sh repl help
./dialtone.sh repl test
./dialtone.sh repl test src_v1
./dialtone.sh repl test src_v1 worktree
./dialtone.sh repl test src_v1 ps
./dialtone.sh repl test src_v1 robot
```

## Subtest Filtering
`repl test` accepts an optional filter argument after version.
It matches test names case-insensitively.

Examples:
- Run only worktree REPL tests:
  - `./dialtone.sh repl test src_v1 worktree`
- Run only `ps` REPL test:
  - `./dialtone.sh repl test src_v1 ps`

## Current Worktree REPL Subtests
- `Test 6: worktree`
  - validates `worktree add`, `worktree list`, `worktree remove`
- `Test 7: worktree start`
  - validates `worktree add` + `worktree start` launch behavior and cleanup

## Notes
- REPL tests inspect subtone logs in `.dialtone/logs`.
- Tests are designed to verify `DIALTONE>` orchestration behavior, not just direct CLI output.
