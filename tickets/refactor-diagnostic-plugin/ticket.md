# Branch: refactor-diagnostic-plugin
# Tags: core, cleanup

# Goal
Refactor `src/diagnostic.go` into a standalone plugin `src/plugins/diagnostic` to improve modularity.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start refactor-diagnostic-plugin`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket refactor-diagnostic-plugin`
- status: done

## SUBTASK: Create Plugin Scaffold and Move Code
- name: create-plugin
- description: Create `src/plugins/diagnostic/{app,cli,test}` and move `src/diagnostic.go` logic into it. Refactor imports and package names.
- test-description: Build the plugin to verify code compilation.
- test-command: `./dialtone.sh plugin build diagnostic`
- status: done

## SUBTASK: Integrate Plugin and Cleanup
- name: integrate-plugin
- description: Update `src/dev.go` to use the new plugin CLI entry point and remove `src/diagnostic.go`.
- test-description: Run the diagnostic command to verify functionality.
- test-command: `./dialtone.sh diagnostic --help`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done refactor-diagnostic-plugin`
- status: todo

