# Merge To Main Checklist (Local-Only)

You are another LLM agent assisting with a local-only merge workflow. Do not start work yet. Follow the steps and record results.

## Subtasks and Commands

### Subtask 1: install-tests
Run install integration tests for PR 166.
```shell
git switch cli-standardization
./dialtone.sh --env test.env install test
```
Verify:
- command exits 0
- install tests report PASS

### Subtask 2: build-tests
Run build integration tests for PR 167.
```shell
git switch build-command-tests
./dialtone.sh build test
```
Verify:
- command exits 0
- build tests report PASS

### Subtask 3: rebase-build-branch
Rebase the build PR branch after PR 166 merges.
```shell
git fetch origin
git switch build-command-tests
git rebase origin/main
git status -sb
```
Verify:
- rebase completes without conflicts
- working tree clean

### Subtask 4: merge-prs
Merge PRs in order: 166 then 167.
```shell
gh pr view 166 --json state,mergedAt
gh pr view 167 --json state,mergedAt
```
Verify:
- both PRs show merged state

## Notes
- All tests are local; no CI checks.
- If a command fails, capture output and stop before proceeding.