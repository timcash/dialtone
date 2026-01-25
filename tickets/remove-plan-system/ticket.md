# Branch: remove-plan-system
# Tags: cleanup, legacy, core

# Goal
Remove the legacy "Plan File" system and the unused `AGENT.md` logic from the repository to simplify the codebase and transition fully to the new ticket system.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start remove-plan-system`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket remove-plan-system`
- status: done

## SUBTASK: remove plan system code
- name: remove-plan-logic
- description: remove `runPlan`, `listPlans`, `countProgress`, `createPlan`, and `showPlan` from `src/dev.go` and unregister the `plan` command.
- test-description: verify the project builds and the `plan` command is unknown.
- test-command: `./dialtone.sh build --local && ./dialtone.sh plan`
- status: done

## SUBTASK: remove AGENT.md and docs command
- name: remove-agent-md
- description: delete `AGENT.md`, remove `runDocs` from `src/dev.go`, and unregister the `docs` command.
- test-description: verify `AGENT.md` is deleted and `docs` command is unknown.
- test-command: `ls AGENT.md && ./dialtone.sh docs`
- status: done

## SUBTASK: update github plugin
- name: update-github-plugin
- description: update `src/plugins/github/cli/github.go` to use the `tickets` directory for PR body generation instead of the `plan` directory.
- test-description: verify the code change in `github.go`.
- test-command: `grep "tickets" src/plugins/github/cli/github.go`
- status: done

## SUBTASK: remove legacy tests
- name: remove-legacy-tests
- description: remove `example_code/old_tests/dialtone-dev-cli/unit_test.go` as it references the removed plan system.
- test-description: verify the file is deleted.
- test-command: `ls example_code/old_tests/dialtone-dev-cli/unit_test.go`
- status: done

## SUBTASK: remove legacy tests
- name: remove-legacy-tests
- description: remove `example_code/old_tests/dialtone-dev-cli/unit_test.go` as it references the removed plan system.
- test-description: verify the file is deleted.
- test-command: `ls example_code/old_tests/dialtone-dev-cli/unit_test.go`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done remove-plan-system`
- status: todo

