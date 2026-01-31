---
title: Nexttone Design
description: Proposed design for the nexttone workflow system
---

# Nexttone Design
This document proposes a new `nexttone` system as its own codebase, using the old
`ticket` plugin only as reference. It uses a microtone workflow to drive
deterministic, signed interactions with great test coverage.

## Goals
- Build a standalone `nexttone` codebase with strong tests.
- Create a new database dedicated to nexttone (no dependency on ticket DB).
- Add a microtone state machine that controls review → execute → complete phases.
- Ensure every prompt includes context and actionable CLI commands.
- Keep the system auditable via signed `--sign yes|no` decisions and logs.

## Non-Goals
- Reuse the existing ticket database or storage tables.
- Provide a broad multi-command CLI; nexttone is intentionally minimal.
- Remove the microtone gating or signed-response requirements.

## Relationship to the Old Ticket System
The `ticket` plugin already provides:
- Ticket and subtask storage (DuckDB).
- A `next` command with TDD-style execution.
- CLI primitives to update subtasks and capture summaries.

For nexttone, the terminology maps as:
- **tone** ≈ ticket
- **subtone** ≈ subtask
- **microtone** ≈ micro-task

Nexttone uses these as reference patterns only. It should **not** share code or DB
with `ticket`; it should re-implement the needed behavior with a clean design,
tests, and its own storage.

## High-Level Architecture
- **CLI Layer**: `./dialtone.sh nexttone` as a single entry that advances the workflow.
- **List View**: `./dialtone.sh nexttone list` for a visual microtone layout with loops.
- **Workflow Engine**: A deterministic state machine that moves through microtones.
- **Prompt Renderer**: Prints context and commands for each microtone question.
- **Storage**: A new nexttone database and tables for microtone state and signatures.

## Proposed CLI Commands
```shell
./dialtone.sh nexttone                         # go to next microtone (default)
./dialtone.sh nexttone next                    # alias of default behavior
./dialtone.sh nexttone list                    # show state machine + loops + current position
./dialtone.sh nexttone add <tone-name>         # add a new tone
./dialtone.sh nexttone subtone add <name>      # add a new subtone
./dialtone.sh nexttone subtone set <name>      # update subtone fields (see below)
```

The `nexttone next` command should accept:
```shell
./dialtone.sh nexttone next --sign yes|no
```

Signing is not optional. `nexttone next` repeats the current microtone question
until it receives a valid `--sign yes|no` response.

## Data Model (Nexttone DB)
Create a dedicated nexttone database with tables such as:
- `nexttone_sessions`: active session, phase, current microtone, timestamps
- `nexttone_microtones`: list of microtones per session, status, ordering
- `nexttone_signatures`: signed responses (yes/no), question, timestamp, context hash
- `nexttone_prompt_logs`: prompt text output for audit and replay
- `nexttone_state_machine`: serialized definition of phases + loops
- `nexttone_microtone_graph`: graph nodes/edges for the microtone workflow

The microtone graph must be stored in this DuckDB database and treated as the
source of truth for state transitions.

## Microtone Workflow (State Machine)
Phases and representative microtones:

### Review Phase
- `set-git-clean`
- `set-git-branch-name`
- `align-goal-subtone-names`
- `review-all-subtones`
- `review-subtone-dependencies-1`
- `review-subtone-dependencies-2`
- `review-subtone-description`
- `review-subtone-test-conditions`
- `review-subtone-test-command`
- `review-subtone-test-outputs`

### Execute Phase
- `start-execute-phase`
- `subtone-ask-test-ready`
- `subtone-run-test`
- `subtone-review-test-logs`
- `subtone-git-commit`
- `subtone-git-push-branch`

### Complete Phase
- `start-complete-phase`
- `document-complete-phase`
- `logs-complete-phase`

Each microtone contains:
- `question`: the user-facing prompt.
- `context`: required data to render before asking.
- `commands`: CLI commands to resolve the question.
- `on_sign_yes`: next microtone or execution action.
- `on_sign_no`: remediation prompt and commands.

## Prompt Rendering Requirements
Every question must include:
- Context (subtone list, current subtone report, git status, etc).
- At least one actionable CLI command.
- Explicit `--sign yes|no` commands (required to proceed).

Prompt format must follow the template in `dialtone_prompt_template.md`.

## Subtone Report Format
Before asking subtone-specific questions, print a structured report:
```shell
# DIALTONE: Subtone Report
# ─────────────────────────────────────────────
# name            : verify-fetch-logic
# description     : ensure fetch returns 200 with valid JSON
# depends-on      : start-server
# test-conditions : response status == 200; body parses JSON
# test-command    : ./dialtone.sh server fetch --url http://localhost:3000/data
# expected-output : status 200; JSON keys: id,name,updated_at
# status          : todo
# last-run        : 2026-01-30 13:24:05
# ─────────────────────────────────────────────
```

## Using the Existing Ticket CLI
The ticket CLI is reference-only. Nexttone should define its own mutation commands
and persistence model within its codebase and database.

## Subtone Mutations (CLI)
Nexttone should support updating subtone fields directly via CLI:
```shell
./dialtone.sh nexttone subtone set <name> --desc "<text>"
./dialtone.sh nexttone subtone set <name> --test-condition "<text>"
./dialtone.sh nexttone subtone set <name> --test-command "<command>"
./dialtone.sh nexttone subtone set <name> --depends-on "<comma-separated>"
./dialtone.sh nexttone subtone set <name> --test-output "<pass/fail details>"
./dialtone.sh nexttone subtone set <name> --agent-notes "<notes>"
```

## Execution Flow (Pseudo-Sequence)
1. `nexttone` / `nexttone next` loads current microtone status.
3. Renderer prints context + question + commands.
4. LLM responds with `--sign yes|no`. If missing or invalid, the same question is reprinted.
5. Engine records signature and advances to next microtone.
6. On execute phase, run the `test-command`, capture output, and ask pass/fail.

## Migration Strategy
- Keep `ticket` intact as a reference-only system.
- Introduce `nexttone` as a separate codebase with its own DB.

## Open Questions
- Should microtone definitions be stored in code, DB, or a markdown spec?
- Do we allow per-tone microtone overrides?
- How should `nexttone` handle tone-level `ask`/`ack` blocks?

## MVP Checklist
- `nexttone` CLI skeleton and command routing.
- Dedicated nexttone database + migrations.
- Microtone list + status storage.
- Prompt renderer with context + commands for each question.
- Signed responses recorded with hashes.
- `nexttone list` shows state machine layout, loops, and current position.
