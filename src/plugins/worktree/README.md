# Worktree Plugin

## Cost Estimate (Gemini 2.5 Flash)
For the default `worktree start`/`worktree test` flow, estimated cost uses:
- input: `$0.30 / 1M` tokens
- output: `$2.50 / 1M` tokens

`worktree test src_v1` now reports and logs actual token counts from Gemini CLI JSON stats and computes:
`estimated_cost_usd = input_tokens*0.30/1e6 + output_tokens*2.50/1e6`

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
./dialtone.sh worktree add <name> --task <file> [--branch <branch>]
./dialtone.sh worktree start <name> [--prompt <text>]
./dialtone.sh worktree tmux-logs <name|index> [-n 10]
./dialtone.sh worktree verify-done <name|index>
./dialtone.sh worktree list
./dialtone.sh worktree attach <name|index>
./dialtone.sh worktree remove <name>
./dialtone.sh worktree cleanup [--all]
./dialtone.sh worktree test src_v1
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
- `test src_v1`
  - runs idempotent E2E: `add -> start -> tmux-logs -> verify-done -> remove`
  - prints token usage + estimated cost
  - emits a structured `[COST] ...` line for REPL/subtone forwarding
  - appends one test record line to this README each run

## REPL Usage
From REPL (`./dialtone.sh`):
- `worktree add ...`
- `worktree start ...`
- `worktree list`
- `worktree tmux-logs ...`
- `worktree verify-done ...`
- `worktree remove ...`

## Test
- 2026-02-20T19:59:26Z | result=PASS | model=gemini-2.5-flash | input=0 output=0 total=0 | estimated_cost_usd=0.000000 | note=ok
- 2026-02-20T20:00:28Z | result=PASS | model=gemini-2.5-flash | input=0 output=0 total=0 | estimated_cost_usd=0.000000 | note=ok
- 2026-02-20T20:02:21Z | result=PASS | model=gemini-2.5-flash | input=0 output=0 total=0 | estimated_cost_usd=0.000000 | note=ok
- 2026-02-20T20:04:09Z | result=PASS | model=gemini-2.5-flash | input=111139 output=855 total=112676 | estimated_cost_usd=0.035479 | note=ok
- 2026-02-20T20:54:58Z | result=PASS | model=gemini-2.5-flash | input=87798 output=700 total=89289 | estimated_cost_usd=0.028089 | note=ok
