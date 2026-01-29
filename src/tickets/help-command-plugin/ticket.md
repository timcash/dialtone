# Name: help-command-plugin
# Tags: plugin, cli, docs

# Goal
Add a help command to a new fictional plugin named `lighthouse`, including usage output and a small test stub.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- tags: setup
- dependencies:
- description: run the cli command `dialtone.sh ticket start help-command-plugin`
- test-condition-1: verify ticket is scaffolded under src/tickets/help-command-plugin
- test-condition-2: ticket is set as current for follow-up commands
- agent-notes: keep this ticket purely local and documentation-driven
- pass-timestamp: 2026-01-28T09:05:10-08:00
- fail-timestamp:
- status: done

## SUBTASK: define lighthouse help behavior
- name: define-help-behavior
- tags: planning, docs
- dependencies: ticket-start
- description: Document the expected `lighthouse` help output, including usage, commands, and examples.
- test-condition-1: `src/plugins/lighthouse/README.md` includes a usage block
- test-condition-2: help output lists at least `help` and `version`
- agent-notes: keep the help text short and example-driven
- pass-timestamp: 2026-01-28T09:12:24-08:00
- fail-timestamp:
- status: done

## SUBTASK: implement lighthouse help command
- name: implement-help-command
- tags: cli
- dependencies: define-help-behavior
- description: Add `./dialtone.sh lighthouse help` (and `--help`) to print the documented usage and examples.
- test-condition-1: running `./dialtone.sh lighthouse help` prints the usage block
- test-condition-2: running `./dialtone.sh lighthouse --help` prints the same output
- agent-notes: reuse a shared help printer in the plugin package
- pass-timestamp:
- fail-timestamp:
- status: progress

## SUBTASK: add lighthouse README
- name: update-plugin-readme
- tags: docs
- dependencies: implement-help-command
- description: Create `src/plugins/lighthouse/README.md` that matches the help output and includes examples.
- test-condition-1: README contains `Usage:` and a command list
- test-condition-2: README example matches `./dialtone.sh lighthouse help`
- agent-notes:
- pass-timestamp:
- fail-timestamp:
- status: todo

## SUBTASK: add plugin test stub
- name: add-cli-test
- tags: test
- dependencies: update-plugin-readme
- description: Add a basic plugin test that asserts help output includes the usage header.
- test-condition-1: `./dialtone.sh lighthouse test` exits cleanly
- test-condition-2: test checks for "Usage:" in help output
- agent-notes:
- pass-timestamp:
- fail-timestamp:
- status: todo

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- tags: cli
- dependencies: add-cli-test
- description: run the ticket cli to verify all steps to complete the ticket
- test-condition-1: validates all ticket subtasks are done
- test-condition-2: `./dialtone.sh ticket done help-command-plugin`
- agent-notes:
- pass-timestamp:
- fail-timestamp:
- status: todo

## Collaborative Notes
- The help command is the first discoverability feature for the plugin, so keep output focused.
