---
title: DIALTONE Prompt Template
description: Standardized prompt format for ticket next workflows
---

# DIALTONE Prompt Template
Use this template to ensure every question includes context and/or CLI commands.
Each prompt must show the next actionable command for the LLM to run.

## General Rules
- Always print context before asking a question.
- Always include at least one CLI command to resolve the question.
- If the LLM already signed (`--sign yes|no`), print the next step immediately.
- Prefer short, structured blocks that are easy to scan.

## Base Prompt (before signature)
```shell
./dialtone.sh ticket next
# DIALTONE [<micro-task-name>]:
# <CONTEXT LINES...>
# DIALTONE: <question>
#   <helpful CLI command>
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
```

## After Signature (guidance)
```shell
./dialtone.sh ticket next --sign <yes|no>
# DIALTONE: <next instruction>
#   <helpful CLI command>
#   ./dialtone.sh ticket next --sign yes
```

## Subtask Report (nice format)
```shell
./dialtone.sh ticket next
# DIALTONE: Subtask Report
# ─────────────────────────────────────────────
# name            : <subtask-name>
# description     : <short goal>
# depends-on      : <comma-separated>
# test-conditions : <conditions>
# test-command    : <command>
# expected-output : <pass/fail details>
# status          : <todo|doing|done>
# last-run        : <timestamp or blank>
# ─────────────────────────────────────────────
# DIALTONE: <question>
#   ./dialtone.sh ticket --subtask <subtask-name> --<field> "<value>"
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
```

## Common Question Templates

### Clean git state
```shell
./dialtone.sh ticket next
# DIALTONE [set-git-clean]:
# GIT STATUS:
# <git status --short output>
# DIALTONE: Is the git clean?
#   git status --short
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
```

### Branch set
```shell
./dialtone.sh ticket next
# DIALTONE [set-git-branch-name]:
# BRANCH: <branch>
# DIALTONE: Is the git branch name set?
#   git branch --show-current
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
```

### Align goal + subtask names
```shell
./dialtone.sh ticket next
# DIALTONE [align-goal-subtask-names]:
# GOAL: <ticket-goal>
# SUBTASKS:
# - <subtask-1>
# - <subtask-2>
# DIALTONE: Is the ticket goal aligned with subtask names?
#   ./dialtone.sh ticket --subtask <subtask-name> --name "<new-name>"
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
```

### Subtask size check
```shell
./dialtone.sh ticket next
# DIALTONE: Subtasks with current time estimates
# - <subtask> (10m)
# - <subtask> (15m)
# DIALTONE: Are any subtasks too large (over ~20 minutes)?
#   ./dialtone.sh ticket subtask add <subtask-name> --desc "<short goal>"
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
```

### Add test-condition
```shell
./dialtone.sh ticket next
# DIALTONE: Test-conditions
# - <condition-1>
# - <condition-2>
# DIALTONE: Are test-conditions concrete and objective?
#   ./dialtone.sh ticket --subtask <subtask-name> --test-condition "<text>"
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
```

### Change test-command
```shell
./dialtone.sh ticket next
# DIALTONE: Current test-command
#   <command>
# DIALTONE: Is test-command present and idempotent?
#   ./dialtone.sh ticket --subtask <subtask-name> --test-command "<command>"
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
```

### Dependencies
```shell
./dialtone.sh ticket next
# DIALTONE: Dependencies
# depends-on: <comma-separated>
# DIALTONE: Are dependencies correct for this subtask?
#   ./dialtone.sh ticket --subtask <subtask-name> --depends-on "<comma-separated>"
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
```

### Expected outputs
```shell
./dialtone.sh ticket next
# DIALTONE: Expected test outputs
# pass: <pass details>
# fail: <fail details>
# DIALTONE: Are expected test outputs documented (pass/fail)?
#   ./dialtone.sh ticket --subtask <subtask-name> --test-output "<pass/fail details>"
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
```

### Execute: test run + result
```shell
./dialtone.sh ticket next --sign yes
# DIALTONE: I am running the test command: <command>
# ... output ...
# exit code: <code>
# DIALTONE: Did the test command pass?
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
```

### Anomalies
```shell
./dialtone.sh ticket next
# DIALTONE: Did you notice any anomalies?
#   ./dialtone.sh ticket --subtask <subtask-name> --agent-notes "<notes>"
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
```

### Commit + push
```shell
./dialtone.sh ticket next
# DIALTONE [subtask-git-commit]:
# CHANGES:
# <git status --short output>
# DIALTONE: Is this subtask ready to commit?
#   git status --short
#   git add <files>
#   git commit -m "<message>"
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
```

```shell
./dialtone.sh ticket next
# DIALTONE [subtask-git-push-branch]:
# BRANCH: <branch>
# AHEAD: <n> commit(s)
# DIALTONE: Should the branch be pushed now?
#   git push -u origin HEAD
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
```
