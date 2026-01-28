# Branch: refactor-dialtone-sh
# Tags: p0, enhancement, ready

# Goal
Refactor `dialtone.sh` to provide a help menu by default and only install Go when the `install` command is explicitly called. Prevent unintended Go installations and ensure a better developer experience.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: run the cli command `dialtone.sh ticket start refactor-dialtone-sh`
- test-description: verify ticket is scaffolded and branch created
- test-command: `./dialtone.sh ticket start refactor-dialtone-sh`
- status: done

## SUBTASK: extract help menu to shell function
- name: extract-help-menu
- description: Move the help message from the Go program into a `print_help` function in `dialtone.sh`.
- test-description: Run `./dialtone.sh` and verify the help menu appears.
- test-command: `./dialtone.sh | grep "Usage: ./dialtone.sh"`
- status: done

## SUBTASK: implement conditional go installation
- name: implement-conditional-install
- description: Modify the script to only perform the Go installation logic if the first argument is `install`.
- test-description: Run a non-install command with a fake env and ensure it doesn't try to download Go.
- test-command: `./dialtone.sh build --env=/tmp/fake_env 2>&1 | grep "Installing..."` (Should not find)
- status: done

## SUBTASK: implement go existence check
- name: implement-go-existence-check
- description: For non-install commands, check if Go exists in the environment path and error if missing.
- test-description: Run `./dialtone.sh build --env=/tmp/fake_env` and verify the error message.
- test-command: `./dialtone.sh build --env=/tmp/fake_env 2>&1 | grep "Error: Go not found"`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket
- test-description: validates all ticket subtasks are done
- test-command: `./dialtone.sh ticket done refactor-dialtone-sh`
- status: done

## Collaborative Notes
- **Context**: [dialtone.sh](file:///Users/dev/code/dialtone/dialtone.sh)
- **Implementation Notes**: 
    - Resolved `DIALTONE_ENV` first to allow correct path checking.
    - Used a `while` loop for argument parsing to correctly identify the command while preserving other options.
    - Integrated help menu directly into the shell script for immediate access without requiring a functional Go environment.
- **Reference**: https://github.com/timcash/dialtone/issues/cli-refactor
