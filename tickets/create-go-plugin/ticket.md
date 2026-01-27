# Branch: create-go-plugin
# Tags: toolchain, go, plugin

# Goal
Create a new Go plugin that handles Go toolchain installation and linting.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start create-go-plugin`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket create-go-plugin`
- status: done

## SUBTASK: scaffold-go-plugin
- name: scaffold-go-plugin
- description: Create the CLI entry point for the go plugin and register it.
- test-description: Verify dialtone.sh go command is recognized.
- test-command: `./dialtone.sh go --help`
- status: done

## SUBTASK: implement-go-install
- name: implement-go-install
- description: Implement the logic to install Go into the DIALTONE_ENV directory.
- test-description: Verify Go is installed in the dependencies directory.
- test-command: `./dialtone.sh go install`
- status: done

## SUBTASK: implement-go-lint
- name: implement-go-lint
- description: Implement the go lint command using the local Go toolchain.
- test-description: Verify go lint runs successfully.
- test-command: `./dialtone.sh go lint`
- status: done

## SUBTASK: documentation
- name: documentation
- description: Update src/plugins/go/README.md with usage instructions.
- test-description: Verify README existence and content.
- test-command: `cat src/plugins/go/README.md`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done create-go-plugin`
- status: done
