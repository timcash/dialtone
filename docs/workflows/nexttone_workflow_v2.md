---
trigger: model_decision
description: nexttone workflow for ticket next subtask loop
---

# OVERVIEW
- `./dialtone.sh ticket` is the primary command for this workflow.
- `LLM` is the LLM agent using the ticket system.
- `DIALTONE` is the workflow driver (the Go code) that prompts the next step.
- The workflow uses only: `ticket start` and `ticket next`.
- There is no `done` in this workflow. The agent stops only when the user says it is done.
- The agent must respond to every `DIALTONE` prompt using `--sign yes|no` to advance the workflow.

## START
```shell
# Given: a ticket name
# About: create a new ticket directory and database entry
./dialtone.sh ticket start <ticket-name>
# New ticket created at `src/tickets/<ticket-name>/`
# Current ticket is set to <ticket-name>
# All future commands default to <ticket-name>
```

## REVIEW (Point-by-Point)
The first `ticket next` enters the review stage and walks a checklist of micro tasks.
Each micro task requires a signed response.

### DIALTONE Must Print Helpful Context
Before asking for a signature, DIALTONE prints what the LLM needs to decide.
This includes current micro-task list, ticket goal, and subtask names.

```shell
./dialtone.sh ticket micro-task list
# DIALTONE: Micro-task list
#    set-git-clean
#      Q1: Is the git clean?
#    set-git-branch-name
#      Q1: Is the git branch name set?
#   >align-goal-subtask-names
#      Q1: Is the ticket goal aligned with subtask names?
#      Q2: Would you change any subtask names?
#      Q3: Is the ticket goal clear and aligned with subtasks?
#    review-all-subtasks
#      Q1: Are any subtasks too large (over ~20 minutes)?
#      Q2: Do any subtasks need splitting or new subtasks added?
#    review-subtask-dependencies-1
#      Q1: Are dependencies correct for this subtask?
#    review-subtask-dependencies-2
#      Q1: Does this subtask need anything before the test is run?
#    review-subtask-description
#      Q1: Is description filled in and aligned with name/goal?
#    review-subtask-test-conditions
#      Q1: Are test-conditions concrete and objective?
#    review-subtask-test-command
#      Q1: Is test-command present and idempotent?
#    review-subtask-test-outputs
#      Q1: Are expected test outputs documented (pass/fail)?
#
#    start-execute-phase
#      Q1: Review complete. Ready to start execution?
#    subtask-ask-test-ready
#      Q1: Is the test ready to be run?
#    subtask-run-test
#      Q1: Did the test command pass?
#    subtask-review-test-logs
#      Q1: Did you notice any anomalies?
#      Q2: Is the test idempotent?
#    subtask-git-commit
#      Q1: Is this subtask ready to commit?
#    subtask-git-push-branch
#      Q1: Should the branch be pushed now?
#
#    start-complete-phase
#      Q1: Ready to start the complete phase?
#    document-complete-phase
#      Q1: Have you documented the complete phase summary?
#    logs-complete-phase
#      Q1: Have you logged the complete phase outputs?
```

```shell
./dialtone.sh ticket next
# DIALTONE [align-goal-subtask-names]:
# GOAL: verify the server can start and fetch logic works
# SUBTASKS:
# - verify-server-running
# - verify-fetch-logic
# - shutdown-server
# DIALTONE: Is the ticket goal aligned with subtask names?
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
./dialtone.sh ticket next --sign no
# DIALTONE: Would you change any subtask names?
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
./dialtone.sh ticket next --sign yes
# DIALTONE: Use the following command to change a name:
#   ./dialtone.sh ticket --subtask <subtask-name> --name "<new-name>"
./dialtone.sh ticket --subtask verify-server-running --name "start-server"
# DIALTONE: The new names are:
# GOAL: verify the server can start and fetch logic works
# SUBTASKS:
# - start-server
# - verify-fetch-logic
# - shutdown-server
# DIALTONE: Would you change any subtask names?
./dialtone.sh ticket next --sign no
# DIALTONE: Is the ticket goal clear and aligned with subtasks?
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
./dialtone.sh ticket next --sign yes
```

```shell
./dialtone.sh ticket next
# DIALTONE: Are any subtasks too large (over ~20 minutes)?
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
./dialtone.sh ticket next --sign no
# DIALTONE: If no, provide a signed command to split or add subtasks:
#   ./dialtone.sh ticket subtask add <subtask-name> --desc "<short goal>"
#   ./dialtone.sh ticket next --sign yes
```

```shell
./dialtone.sh ticket next
# DIALTONE: Do any subtasks need splitting or new subtasks added?
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
./dialtone.sh ticket next --sign yes
# Example: add a new subtask
# ./dialtone.sh ticket subtask add <subtask-name> --desc "<short goal>"
# Then sign the change:
# ./dialtone.sh ticket next --sign yes
```

### Subtask Field Micro Tasks (with --sign)
For every subtask, the review checks each field.

```shell
./dialtone.sh ticket next
# DIALTONE: Current subtask (readable format)
# name: start-server
# description: start the server
# test-condition-1: the server is running
# test-command: ./dialtone.sh server start
# pass-timestamp:
# fail-timestamp:
# status: todo
# agent-notes:
# DIALTONE: Is description filled in?
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
./dialtone.sh ticket next --sign yes
```

```shell
./dialtone.sh ticket next
# DIALTONE: Are test-conditions concrete and objective?
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
./dialtone.sh ticket next --sign no
# DIALTONE: Use the following command to change test-conditions:
#   ./dialtone.sh ticket --subtask <subtask-name> --test-condition "<text>"
# Fix test-conditions then re-run:
# ./dialtone.sh ticket next --sign yes
```

```shell
./dialtone.sh ticket next
# DIALTONE: Is test-command present and idempotent?
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
./dialtone.sh ticket next --sign yes
# DIALTONE: Use the following command to change test-command:
#   ./dialtone.sh ticket --subtask <subtask-name> --test-command "<command>"
# Example signed proof:
# <test-command>
# ./dialtone.sh ticket next --sign yes
```

```shell
./dialtone.sh ticket next
# DIALTONE: Are dependencies correct for this subtask?
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
./dialtone.sh ticket next --sign yes
```

## EXECUTE (After Review)
Once review is complete, `ticket next` enters execution mode.
The workflow then works **one subtask at a time**.

```shell
./dialtone.sh ticket next
# DIALTONE: Review complete. Fetching subtask to execute...
# name: start-server
# description: start the server
# test-condition-1: the server is running
# test-command: ./dialtone.sh server start
# pass-timestamp:
# fail-timestamp:
# status: todo
# agent-notes:
#
# DIALTONE: Is the test ready to be run?
# DIALTONE: Edit src/tickets/<ticket-name>/test.go with any changes needed
./dialtone.sh ticket next --sign yes
# DIALTONE: I am running the test command: ./dialtone.sh server start
# ... output of the test command....
# ... server is running...
# ... crash ERROR ...
# exit code: 1
# DIALTONE: Did the test command pass?
#   ./dialtone.sh ticket next --sign no|yes
./dialtone.sh ticket next --sign no
# DIALTONE: Would you change the test command?
# DIALTONE: Update the test code and run next to try again
#   ./dialtone.sh ticket next --sign no|yes
./dialtone.sh ticket next --sign yes
# DIALTONE: I am running the test command: ./dialtone.sh server start
# ... output of the test command....
# ... server is running...
# ... success ...
# exit code: 0
# DIALTONE: Did the test command pass?
#   ./dialtone.sh ticket next --sign no|yes
./dialtone.sh ticket next --sign yes
# DIALTONE: Did you notice any anomalies?
#   ./dialtone.sh ticket next --sign no|yes
./dialtone.sh ticket next --sign no
# DIALTONE: Is the test idempotent?
#   ./dialtone.sh ticket next --sign no|yes
./dialtone.sh ticket next --sign yes
```

### Execute One Subtask (with --sign)
```shell
# Run the test-command
<test-command>

# Sign the result
./dialtone.sh ticket next --sign yes
```

If the test fails (signed):
```shell
# Capture why it failed
./dialtone.sh ticket next --sign no
```

### Example: Signed Verification (full loop)
```shell
./dialtone.sh ticket next
# DIALTONE: Is the test-command idempotent?
./dialtone.sh ticket next --sign yes

./dialtone.sh ticket next
# DIALTONE: Did the test-command pass?
./dialtone.sh ticket next --sign yes

./dialtone.sh ticket next
# DIALTONE: Is this subtask ready to move forward?
./dialtone.sh ticket next --sign yes
```

### Example: New Question from DIALTONE (signed)
```shell
./dialtone.sh ticket next
# DIALTONE: Is a new subtask required for documentation?
./dialtone.sh ticket next --sign no
```

## Working Rules During Execute
- The LLM may edit test-conditions, add summaries, debug code, and add new subtasks.
- The LLM must re-run the test-command after changes.
- Each `--sign yes` must be backed by an executed command.

## Ticket Completion Gate (No Done Command)
When all subtasks are complete, DIALTONE asks if the PR is ready.
The LLM must sign off on git status and that all code is committed **per subtask**.

```shell
./dialtone.sh ticket next
# DIALTONE: Is the PR ready and all changes committed?
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
./dialtone.sh ticket next --sign yes

./dialtone.sh ticket next
# DIALTONE: Sign off git status per subtask (clean + committed)
#   ./dialtone.sh ticket next --sign no
#   ./dialtone.sh ticket next --sign yes
./dialtone.sh ticket next --sign yes
```