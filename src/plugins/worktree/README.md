# Worktree Plugin

## Purpose
The `worktree` plugin runs isolated agent workflows using:
- Git worktrees
- detached tmux sessions
- task-signature status in `TASK.md`

Managed worktree base directory:
- `/home/user/dialtone_worktree`

## Workflow
1. `add`: create worktree + tmux session + copy task to `TASK.md`
2. `start`: launch Gemini CLI in tmux and stream pane output to `<worktree>/tmux.log`
3. `tmux-logs`: inspect latest session output
4. `verify-done`: validate `TASK.md` signature is `done` and (for agent_test) verify command passes
5. `remove`: kill tmux session and remove worktree folder

## CLI
```bash
./dialtone.sh worktree src_v1 add <name> --task <file> [--branch <branch>]
./dialtone.sh worktree src_v1 start <name> [--prompt <text>]
./dialtone.sh worktree src_v1 tmux-logs <name|index> [-n 10]
./dialtone.sh worktree src_v1 verify-done <name|index>
./dialtone.sh worktree src_v1 list
./dialtone.sh worktree src_v1 attach <name|index>
./dialtone.sh worktree src_v1 remove <name>
./dialtone.sh worktree src_v1 cleanup [--all]
./dialtone.sh worktree src_v1 test
```

## Command Notes
- `add`
  - creates worktree at `/home/user/dialtone_worktree/<name>`
  - starts tmux session `<name>`
  - copies full `env/.env` and normalizes `DIALTONE_ENV`
- `start`
  - requires existing worktree + tmux session + `TASK.md`
  - default prompt tells agent to complete `TASK.md` and sign before work
  - writes tmux stream to `<worktree>/tmux.log`
- `list`
  - shows index, name, task status (`wait|work|done|fail`), tmux state, branch, path
- `cleanup`
  - `cleanup`: removes stale managed worktrees
  - `cleanup --all`: removes all non-root worktrees (including legacy locations)
- `test`
  - runs `src/plugins/worktree/src_v1/test/cmd/main.go`
  - uses shared `testv1` registry with numbered suite folders
  - currently verifies CLI order/help and old-order warning behavior

## REPL Usage
From REPL (`./dialtone.sh`):
- `worktree src_v1 add ...`
- `worktree src_v1 start ...`
- `worktree src_v1 list`
- `worktree src_v1 tmux-logs ...`
- `worktree src_v1 verify-done ...`
- `worktree src_v1 remove ...`
