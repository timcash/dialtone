---
title: DIALTONE Prompt Template
description: Standardized prompt format for nexttone workflows
---

# DIALTONE Prompt Template
Use this template to ensure every question includes context and/or CLI commands.
Each prompt must show the next actionable command for the LLM to run.

## General Rules
- Always print context before asking a question.
- Always include at least one CLI command to resolve the question.
- If the LLM already signed (`--sign yes|no`), print the next step immediately.
- Prefer short, structured blocks that are easy to scan.

## Base Prompt (single pattern)
```shell
./dialtone.sh nexttone
# DIALTONE [<microtone-name>]:
# <CONTEXT LINES...>
# DIALTONE: <question>
#   <helpful CLI command>
#   ./dialtone.sh nexttone --sign no
#   ./dialtone.sh nexttone --sign yes
```

## Subtone Report (nice format)
```shell
./dialtone.sh nexttone
# DIALTONE: Subtone Report
# ─────────────────────────────────────────────
# name            : <subtone-name>
# description     : <short goal>
# depends-on      : <comma-separated>
# test-conditions : <conditions>
# test-command    : <command>
# expected-output : <pass/fail details>
# status          : <todo|doing|done>
# last-run        : <timestamp or blank>
# ─────────────────────────────────────────────
# DIALTONE: <question>
#   ./dialtone.sh nexttone subtone set <subtone-name> --<field> "<value>"
#   ./dialtone.sh nexttone --sign no
#   ./dialtone.sh nexttone --sign yes
```

## Common Question Templates

### Clean git state
```shell
./dialtone.sh nexttone
# DIALTONE [set-git-clean]:
# GIT STATUS:
# <git status --short output>
# DIALTONE: Is the git clean?
#   git status --short
#   ./dialtone.sh nexttone --sign no
#   ./dialtone.sh nexttone --sign yes
```

### Branch set
```shell
./dialtone.sh nexttone
# DIALTONE [set-git-branch-name]:
# BRANCH: <branch>
# DIALTONE: Is the git branch name set?
#   git branch --show-current
#   ./dialtone.sh nexttone --sign no
#   ./dialtone.sh nexttone --sign yes
```

### Align goal + subtone names
```shell
./dialtone.sh nexttone
# DIALTONE [align-goal-subtone-names]:
# GOAL: <tone-goal>
# SUBTONES:
# - <subtone-1>
# - <subtone-2>
# DIALTONE: Is the tone goal aligned with subtone names?
#   ./dialtone.sh nexttone subtone set <subtone-name> --name "<new-name>"
#   ./dialtone.sh nexttone --sign no
#   ./dialtone.sh nexttone --sign yes
```

### Subtone size check
```shell
./dialtone.sh nexttone
# DIALTONE: Subtones with current time estimates
# - <subtone> (10m)
# - <subtone> (15m)
# DIALTONE: Are any subtones too large (over ~20 minutes)?
#   ./dialtone.sh nexttone subtone add <subtone-name> --desc "<short goal>"
#   ./dialtone.sh nexttone --sign no
#   ./dialtone.sh nexttone --sign yes
```

### Add test-condition
```shell
./dialtone.sh nexttone
# DIALTONE: Test-conditions
# - <condition-1>
# - <condition-2>
# DIALTONE: Are test-conditions concrete and objective?
#   ./dialtone.sh nexttone subtone set <subtone-name> --test-condition "<text>"
#   ./dialtone.sh nexttone --sign no
#   ./dialtone.sh nexttone --sign yes
```

### Change test-command
```shell
./dialtone.sh nexttone
# DIALTONE: Current test-command
#   <command>
# DIALTONE: Is test-command present and idempotent?
#   ./dialtone.sh nexttone subtone set <subtone-name> --test-command "<command>"
#   ./dialtone.sh nexttone --sign no
#   ./dialtone.sh nexttone --sign yes
```

### Dependencies
```shell
./dialtone.sh nexttone
# DIALTONE: Dependencies
# depends-on: <comma-separated>
# DIALTONE: Are dependencies correct for this subtone?
#   ./dialtone.sh nexttone subtone set <subtone-name> --depends-on "<comma-separated>"
#   ./dialtone.sh nexttone --sign no
#   ./dialtone.sh nexttone --sign yes
```

### Expected outputs
```shell
./dialtone.sh nexttone
# DIALTONE: Expected test outputs
# pass: <pass details>
# fail: <fail details>
# DIALTONE: Are expected test outputs documented (pass/fail)?
#   ./dialtone.sh nexttone subtone set <subtone-name> --test-output "<pass/fail details>"
#   ./dialtone.sh nexttone --sign no
#   ./dialtone.sh nexttone --sign yes
```

### Execute: test run + result
```shell
./dialtone.sh nexttone --sign yes
# DIALTONE: I am running the test command: <command>
# ... output ...
# exit code: <code>
# DIALTONE: Did the test command pass?
#   ./dialtone.sh nexttone --sign no
#   ./dialtone.sh nexttone --sign yes
```

### Anomalies
```shell
./dialtone.sh nexttone
# DIALTONE: Did you notice any anomalies?
#   ./dialtone.sh nexttone subtone set <subtone-name> --agent-notes "<notes>"
#   ./dialtone.sh nexttone --sign no
#   ./dialtone.sh nexttone --sign yes
```

### Commit + push
```shell
./dialtone.sh nexttone
# DIALTONE [subtone-git-commit]:
# CHANGES:
# <git status --short output>
# DIALTONE: Is this subtone ready to commit?
#   git status --short
#   git add <files>
#   git commit -m "<message>"
#   ./dialtone.sh nexttone --sign no
#   ./dialtone.sh nexttone --sign yes
```

```shell
./dialtone.sh nexttone
# DIALTONE [subtone-git-push-branch]:
# BRANCH: <branch>
# AHEAD: <n> commit(s)
# DIALTONE: Should the branch be pushed now?
#   git push -u origin HEAD
#   ./dialtone.sh nexttone --sign no
#   ./dialtone.sh nexttone --sign yes
```
