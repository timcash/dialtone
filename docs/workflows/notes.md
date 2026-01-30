---
trigger: model_decision
description: nexttone workflow for ticket next subtask loop
---

# OVERVIEW:
- `./dialtone.sh ticket` is the primary command for this workflow
- `LLM` is the LLM agent using the `./dialtone.sh ticket` system
- `LLM` must answer all questions from the `DAILTONE` user
- `DIALTONE` must be clear and concise in its questions
- `LLM` Must not make assumptions about the user's intent
- `LLM` Must ask for help or clarification if needed
- `LLM` Must suggest alternatives if recognized as better way
- `LLM` Must sign work only after carefully considering the task
- `LLM` To create new steps use this format:
```markdown
# <step-name>:
```shell
# Given: context about the system state before the command
# About: what the command does
./dialtone.sh <command> <args>
# EXAMPLE OUTPUT:
# <example output>
# ...
```

# START:
```shell
# Given: a ticket name
# About: create a new ticket directory and database entry
./dialtone.sh ticket start <ticket-name>
# New ticket created at `src/tickets/<ticket-name>/`
# Current ticket is set to <ticket-name> 
# All furture commands will deafult to <ticket-name>
```

# REVIEW:
```shell
./dialtone.sh ticket micro-task list
#    set-git-clean
#    set-git-branch-name
#   >align-goal-subtask-names
#    review-all-subtasks
#    review-subtask-dependencies
#    review-subtask-description
#    review-subtask-test-conditions
#    review-subtask-test-command
#    review-subtask-test-outputs
#
#    start-execute-phase
#    subtask-ask-test-ready
#    subtask-run-test
#    subtask-review-test-logs
#    subtask-git-commit
#    subtask-git-push-branch
#
#    start-complete-phase
#    document-complete-phase
#    logs-complete-phase

./dialtone.sh ticket next
# DIALTONE [align-goal-subtask-names]: is the ticket goal algined with subtask names?
# GOAL: verify the server can start and fetch logic works
# SUBTASKS:
# verify-server-running
# verify-fetch-logic
# shutdown-server
# DIALTONE: use the one of the follow commands to sign
#         ./dialtone.sh ticket next --sign no|yes
./dialtone.sh ticket next --sign no
# DIALTONE: Would you change any subtask names?
#         ./dialtone.sh ticket next --sign no|yes
./dialtone.sh ticket next --sign yes
# DIALTONE: Use the following command to change a name:
#         ./dialtone.sh ticket --subtask <subtask-name> --name "<new-name>"
./dialtone.sh ticket --subtask verify-server-running --name "start-server"
# DIALTONE: the new names are:
# GOAL: verify the server can start and fetch logic works
# SUBTASKS:
# start-server
# verify-fetch-logic
# shutdown-server
# DIALTONE: Would you change any subtask names?
./dialtone.sh ticket next --sign no
# DIALTONE: Would you split any subtasks into smaller subtasks?

```

# EXECUTE:
```shell
./dialtone.sh ticket next
# DIALTONE: Review is complete
# Fetching subtask to execute: start-server
# name: start-server
# description: start the server
# test-condition-1: the server is running
# test-command: ./dialtone.sh server start
#
# DIALTONE: is the test ready to be run? 
# DIALTONE: edit src/tickets/<ticket-name>/test.go with any changes needed 
./dialtone.sh ticket next --sign yes
# DIALTONE: I am running the test command: ./dialtone.sh server start
# ... output of the test command....
# ... server is running...
# ... crash ERROR ...
# exit code: 1
# DIALTONE: did the test command pass?
#         ./dialtone.sh ticket next --sign no|yes
./dialtone.sh ticket next --sign no
# DIALTONE: Would you change the test command?
# DIALTONE: Update the test code and run next to try again
#         ./dialtone.sh ticket next --sign no|yes
./dialtone.sh ticket next --sign yes
# DIALTONE: I am running the test command: ./dialtone.sh server start
# ... output of the test command....
# ... server is running...
# ... success ...   
# exit code: 0
# DIALTONE: did the test command pass?
#         ./dialtone.sh ticket next --sign no|yes
./dialtone.sh ticket next --sign yes
# DIALTONE: Did you notice any anaomolies?
#         ./dialtone.sh ticket next --sign no|yes
./dialtone.sh ticket next --sign no
# DIALTONE: Is the test idempotent?
#         ./dialtone.sh ticket next --sign no|yes
./dialtone.sh ticket next --sign yes















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
#    set-git-clean                      : Is the git clean?
#    set-git-branch-name                : Is the git branch name set?
#   >align-goal-subtask-names           : Is the ticket goal clear and aligned with subtasks?
#    review-all-subtasks                : Are all subtasks now clear and ready to be executed?
#    review-subtask-dependencies-1      : Are the subtask dependencies correct?
#    review-subtask-dependencies-2      : Does it need anything before the test is run?
#    review-subtask-description         : Does the description align with name and ticket goal?
#    review-subtask-test-conditions     : Are the test conditions clear and objective?
#    review-subtask-test-command        : Is the test command present and idempotent?
#    review-subtask-test-outputs
#
#    start-execute-phase
#    subtask-ask-test-ready
#    subtask-run-test
#    subtask-review-test-logs
#    subtask-git-commit
#    subtask-git-push-branch
#
#    start-complete-phase
#    document-complete-phase
#    logs-complete-phase
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
