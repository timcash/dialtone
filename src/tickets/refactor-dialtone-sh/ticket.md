# Branch: refactor-dialtone-sh
# Tags: p0, enhancement, ready

# Goal
Refactor `dialtone.sh` to provide a help menu by default and only install Go when the `install` command is explicitly called. Prevent unintended Go installations and ensure a better developer experience.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- tags: cli
- dependencies: 
- description: run the cli command `dialtone.sh ticket start refactor-dialtone-sh`
- test-condition-1: verify ticket is scaffolded
- test-condition-2: verify branch created
- agent-notes:
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: extract help menu to shell function
- name: extract-help-menu
- tags: shell
- dependencies: ticket-start
- description: Move the help message from the Go program into a `print_help` function in `dialtone.sh`.
- test-condition-1: Run `./dialtone.sh` and verify the help menu appears.
- test-condition-2: regex check for Usage string
- agent-notes:
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: implement conditional go installation
- name: implement-conditional-install
- tags: logic
- dependencies: extract-help-menu
- description: Modify the script to only perform the Go installation logic if the first argument is `install`.
- test-condition-1: Run a non-install command with a fake env and ensure it doesn't try to download Go.
- test-condition-2: check for absence of "Installing..." string
- agent-notes:
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: implement go existence check
- name: implement-go-existence-check
- tags: logic
- dependencies: implement-conditional-install
- description: For non-install commands, check if Go exists in the environment path and error if missing.
- test-condition-1: Run `./dialtone.sh build --env=/tmp/fake_env` and verify the error message.
- test-condition-2: check for "Error: Go not found"
- agent-notes:
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- tags: cli
- dependencies: implement-go-existence-check
- description: run the ticket cli to verify all steps to complete the ticket
- test-condition-1: validates all ticket subtasks are done
- test-condition-2: verifies git status clean
- agent-notes:
- pass-timestamp: 
- fail-timestamp: 
- status: done

## Collaborative Notes
- **Context**: [dialtone.sh](file:///Users/dev/code/dialtone/dialtone.sh)
- **Implementation Notes**: 
    - Resolved `DIALTONE_ENV` first to allow correct path checking.
    - Used a `while` loop for argument parsing to correctly identify the command while preserving other options.
    - Integrated help menu directly into the shell script for immediate access without requiring a functional Go environment.
- **Reference**: https://github.com/timcash/dialtone/issues/cli-refactor
