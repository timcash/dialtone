# Branch: help-command-plugin
# Tags: docs, plugin, cli

# Goal
Add a help command to a new fictional plugin named `lighthouse` so users can discover commands and usage quickly.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start help-command-plugin`
- test-condition-1: verify ticket is scaffolded under src/tickets/help-command-plugin
- test-condition-2: `./dialtone.sh plugin test ticket`
- tags: setup
- dependencies: 
- agent-notes: scoped to a new plugin with only help output
- pass-timestamp: 2026-01-28T09:05:10-08:00
- fail-timestamp: 
- status: done

## SUBTASK: define lighthouse help behavior
- name: define-help-behavior
- description: Document the expected `lighthouse` help output, including usage, commands, and examples.
- test-condition-1: `src/plugins/lighthouse/README.md` includes a usage block
- test-condition-2: help output lists at least `help` and `version`
- tags: planning, docs
- dependencies: ticket-start
- agent-notes: keep the help text short and example-driven
- pass-timestamp: 2026-01-28T09:12:24-08:00
- fail-timestamp: 
- status: done

## SUBTASK: implement lighthouse help command
- name: implement-help-command
- description: Add `./dialtone.sh lighthouse help` (and `--help`) to print the documented usage and examples.
- test-condition-1: running `./dialtone.sh lighthouse help` prints the usage block
- test-condition-2: running `./dialtone.sh lighthouse --help` prints the same output
- tags: cli
- dependencies: define-help-behavior
- agent-notes: reuse a shared help printer in the plugin package
- pass-timestamp: 
- fail-timestamp: 
- status: progress

## SUBTASK: add lighthouse README
- name: update-plugin-readme
- description: Create `src/plugins/lighthouse/README.md` that matches the help output and includes examples.
- test-condition-1: README contains `Usage:` and a command list
- test-condition-2: README example matches `./dialtone.sh lighthouse help`
- tags: docs
- dependencies: implement-help-command
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: todo

## SUBTASK: add plugin test stub
- name: add-cli-test
- description: Add a basic plugin test that asserts help output includes the usage header.
- test-condition-1: `./dialtone.sh lighthouse test` exits cleanly
- test-condition-2: test checks for "Usage:" in help output
- tags: test
- dependencies: update-plugin-readme
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: todo

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket
- test-condition-1: validates all ticket subtasks are done
- test-condition-2: `./dialtone.sh ticket done help-command-plugin`
- tags: cli
- dependencies: add-cli-test
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: todo

## Collaborative Notes
- The help command is the first discoverability feature for the plugin, so keep output focused.
