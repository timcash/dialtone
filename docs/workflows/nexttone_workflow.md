---
trigger: model_decision
description: nexttone workflow for ticket next subtask loop
---

# Workflow: Nexttone (Ticket Next Loop)

Nexttone is the step-by-step workflow for LLM agents using the `./dialtone.sh ticket` system.
Its purpose is to guide the agent through one subtask at a time, ensuring each subtask has
clear test conditions, an idempotent test command, and a traceable report.

## Principles
- **One subtask at a time**: use `ticket next` to drive the queue.
- **Tests before done**: a subtask is only complete if its test command passes.
- **Idempotent verification**: test commands must be safe to rerun.
- **Traceability**: every claim is signed with a command and logged.
- **Scope control**: split large subtasks or create side-quest tickets.

## Start
```bash
./dialtone.sh ticket start <ticket-name>
./dialtone.sh ticket subtask list
```

## The `ticket next` contract
Whenever `ticket next` is run, the agent should confirm the following before proceeding:

1. **Summary freshness**
   - If last summary is stale, run:
     ```bash
     ./dialtone.sh ticket summary update
     ```

2. **Subtask field completeness**
   - Each subtask must have:
     - `description`
     - at least one `test-condition`
     - a `test-command` (idempotent)
   - If any are missing, update the ticket first (do not run tests yet).

3. **Subtask quality checks**
   - Is the subtask small enough to finish in ~20 minutes?
   - Are test conditions objective and verifiable?
   - Is the test command safe to run multiple times?
   - If not, split or rewrite the subtask.

4. **Run the subtask test**
   ```bash
   ./dialtone.sh ticket next
   ```

5. **Sign the result**
   - If it passed, record a signed statement:
     ```bash
     ./dialtone.sh ticket log "SIGN: <subtask> test-command is idempotent and PASS"
     ```
   - If it failed, capture the reason and plan:
     ```bash
     ./dialtone.sh ticket log "FAIL: <subtask> <reason> (next action: ...)"
     ```

## Subtask checklist (before `ticket next`)
- **Test conditions** are concrete:
  - good: "command exits 0"
  - bad: "looks correct"
- **Test command** is present and idempotent:
  - good: `./dialtone.sh build test`
  - bad: `rm -rf ~/.config` or non-repeatable actions
- **Subtask scope** is reasonable:
  - If too large, split into smaller subtasks.

## When to split a subtask
Split when any of the following are true:
- It spans multiple modules or teams
- It exceeds ~20 minutes to implement or verify
- It requires separate tests for separate concerns

Use:
```bash
./dialtone.sh ticket subtask add <new-subtask-name> --desc "<short goal>"
```

## When to create a side-quest ticket
Create a side-quest ticket when:
- You discover unrelated bugs or refactors
- It blocks the current subtask but is not part of acceptance criteria

Use:
```bash
./dialtone.sh ticket add <side-quest-name>
./dialtone.sh ticket ask "Created side-quest <side-quest-name> for <issue>"
```

## Capturing questions and decisions
```bash
./dialtone.sh ticket ask "<question>"
./dialtone.sh ticket ack "<answer>"
./dialtone.sh ticket log "<decision>"
```

## Example: One subtask loop
```bash
# See current queue
./dialtone.sh ticket subtask list

# Run next subtask test
./dialtone.sh ticket next

# Sign the outcome
./dialtone.sh ticket log "SIGN: install-tests test-command is idempotent and PASS"
```

## Example: Subtask field checklist
```bash
./dialtone.sh ticket log "CHECK: install-tests has description, test-conditions, test-command"
```

## Finalize
```bash
./dialtone.sh ticket test <ticket-name>
./dialtone.sh ticket validate <ticket-name>
./dialtone.sh ticket done
```
