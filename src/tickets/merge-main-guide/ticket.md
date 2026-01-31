# Merge Main Guide

## Description
Run the local test suite for both PRs, rebase the second PR on the updated main branch, and merge both PRs into `main`.

## Subtasks

### 1) install-tests
Run the install integration tests for PR 166.

Test command:
```shell
git switch cli-standardization
./dialtone.sh --env test.env install test
```

Pass criteria:
- Command exits 0
- Install integration tests report PASS

### 2) build-tests
Run the build integration tests for PR 167.

Test command:
```shell
git switch build-command-tests
./dialtone.sh build test
```

Pass criteria:
- Command exits 0
- Build integration tests report PASS

### 3) rebase-build-branch
Rebase PR 167 on the updated `main` after PR 166 is merged.

Test command:
```shell
git fetch origin
git switch build-command-tests
git rebase origin/main
git status -sb
```

Pass criteria:
- Rebase completes without conflicts
- Working tree is clean

### 4) merge-prs
Merge the PRs in sequence: PR 166 first, then PR 167.

Test command:
```shell
gh pr view 166 --json state,mergedAt
gh pr view 167 --json state,mergedAt
```

Pass criteria:
- Both PRs show merged state
