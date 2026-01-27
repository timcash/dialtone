# Branch: replace-progress-with-ticket-next
# Tags: refactor, documentation, ticket-system

# Goal
Replace the manual `progress.txt` tracking with a centralized and automated `dialtone.sh ticket next` command. This command will become the primary driver for ticket execution, handling validation, test execution, and state transitions.

## SUBTASK: Implement root `ticket next` command
- name: implement-ticket-next-cmd
- description: Add the `next` subcommand to `Run` in `src/plugins/ticket/cli/ticket.go` and its entry point in `subtask.go`. It should check if on `main`/`master` and error/instruct agents to ask the user for a ticket.
- test-description: Run `./dialtone.sh ticket next` on main and verify the error message.
- test-command: `./dialtone.sh ticket next`
- status: todo

## SUBTASK: Add validation and state transitions to `ticket next`
- name: ticket-next-logic
- description: Update `RunSubtaskNext` to: 1. Validate the ticket format. 2. If a subtask was in `progress`, mark it `done` (if tests pass). 3. Identify and print the next `todo` subtask, marking it `progress`.
- test-description: Verify that `ticket next` moves a subtask from `todo` to `progress` and eventually to `done`.
- test-command: `./dialtone.sh ticket subtask next`
- status: todo

## SUBTASK: Integrate test execution into `ticket next`
- name: ticket-next-tests
- description: Ensure `ticket next` runs all registered tests for the ticket (equivalent to `test.RunTicket`) before allowing state transitions.
- test-description: Verify tests are executed when running `ticket next`.
- test-command: `./dialtone.sh ticket next`
- status: todo

## SUBTASK: Remove `progress.txt` and associated validation
- name: remove-progress-txt
- description: Delete `progress.txt` from the project. Remove `validateGitState` logic that enforces `progress.txt` updates in `subtask.go`. Update `ScaffoldTicket` to no longer create the file.
- test-description: Verify `progress.txt` is no longer created or required by any command.
- test-command: `ls tickets/replace-progress-with-ticket-next/progress.txt`
- status: todo

## SUBTASK: Update workflows to mandate `ticket next`
- name: update-workflow-docs
- description: Update `docs/workflows/ticket.md` and other workflows to replace `subtask done` and `progress.txt` references with the mandatory `ticket next` flow. Include a simple bash block guiding the through this process.
- test-description: Grep for `progress.txt` in `docs/workflows/` and ensure it's gone.
- test-command: `grep -r "progress.txt" docs/workflows/`
- status: todo

## SUBTASK: Update READMEs and CLI documentation
- name: update-readme-docs
- description: Update `src/plugins/ticket/README.md` and other relevant docs to reflect the new `ticket next` behavior.
- test-description: Verify README content.
- test-command: `cat src/plugins/ticket/README.md`
- status: todo

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: run the cli command `dialtone.sh ticket start replace-progress-with-ticket-next`
- test-description: verify ticket is scaffolded and branch created
- test-command: `dialtone.sh test ticket replace-progress-with-ticket-next`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket
- test-description: validates all ticket subtasks are done
- test-command: `dialtone.sh ticket done replace-progress-with-ticket-next`
- status: todo

## Collaborative Notes
- The goal is to make `ticket.md` the single source of truth for both state and testing.
- agents should be instructed that `ticket next` is their primary loop.
