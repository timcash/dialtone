---
title: Nexttone CLI
description: Microtone workflow driver for Dialtone
---

# Nexttone CLI (beta)
Nexttone is a minimal workflow driver that advances a microtone state machine.
Every command that changes state prints a `DIALTONE` prompt so the LLM always
sees context plus the next actionable commands.

## Commands
```bash
./dialtone.sh nexttone                   # show current microtone prompt
./dialtone.sh nexttone next              # alias of default behavior
./dialtone.sh nexttone list              # show microtone graph + subtone list
./dialtone.sh nexttone add <tone-name>   # add a tone and scaffold test
./dialtone.sh nexttone subtone add <name> [--desc "..."]
./dialtone.sh nexttone subtone set <name> --<field> "..."
./dialtone.sh nexttone --sign yes|no     # record signature and advance
./dialtone.sh nexttone help              # show help menu
```

Tone names must be 3 to 5 kebab-case words (example: `nexttone-graph-demo`).

## Tone folders
When you add a tone, Nexttone creates:
```
src/nexttone/<tone-name>/
├── <tone-name>.duckdb
└── test/
    └── test.go
```

### Subtone fields
`subtone set` supports:
- `--name`
- `--desc`
- `--test-condition` (repeatable to set multiple conditions)
- `--test-command`
- `--depends-on`
- `--test-output`
- `--agent-notes`

## DIALTONE prompt behavior
After each command, Nexttone prints a structured prompt:
- Context first
- A question line starting with `DIALTONE:`
- One or more actionable CLI commands
- Explicit `--sign yes|no` commands when advancing microtones

This keeps the LLM aligned with the next required action.

## Environment
```bash
NEXTTONE_DB_PATH  # override nexttone DB path (default: src/nexttone/<tone>/nexttone.duckdb)
NEXTTONE_TONE     # active tone name (default: default)
NEXTTONE_TONE_DIR # tone root directory (default: src/nexttone)
```

## Examples
```bash
./dialtone.sh nexttone
./dialtone.sh nexttone --sign yes
./dialtone.sh nexttone add www-nexttone-section
./dialtone.sh nexttone subtone add nexttone-graph --desc "add graph viz"
./dialtone.sh nexttone subtone set nexttone-graph --test-command "./dialtone.sh plugin test nexttone"
./dialtone.sh nexttone list
```
