# Branch: update-ticket-plugin-readme
# Tags: <tags>

# Goal
Update `src/plugins/ticket/README.md` to be the primary documentation for the `ticket` plugin, including bash code blocks for CLI commands, examples, and a subtask documentation section.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start update-ticket-plugin-readme`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket update-ticket-plugin-readme`
- status: done

## SUBTASK: consolidate information and draft structure
- name: consolidate-and-draft
- description: Review `docs/cli.md`, `docs/ticket-template.md`, and `src/plugins/ticket/cli/` code. Draft the initial structure for the new `README.md` with sections for core commands, subtask commands, and the subtask format.
- test-description: Verify the drafted structure exists in `src/plugins/ticket/README.md`.
- test-command: `ls src/plugins/ticket/README.md`
- status: done

## SUBTASK: document core ticket commands
- name: document-core-commands
- description: Add detailed bash code blocks and descriptions for `add`, `start`, `done`, `list`, and `validate` subcommands.
- test-description: Verify bash blocks exist for each core command in the README.
- test-command: `grep -E "ticket (add|start|done|list|validate)" src/plugins/ticket/README.md`
- status: done

## SUBTASK: document subtask management commands
- name: document-subtask-commands
- description: Add detailed bash code blocks and descriptions for `subtask list`, `subtask next`, `subtask test`, and `subtask done`.
- test-description: Verify bash blocks exist for each subtask command in the README.
- test-command: `grep -E "ticket subtask (list|next|test|done)" src/plugins/ticket/README.md`
- status: done

## SUBTASK: document subtask format and TDD workflow
- name: document-subtask-format
- description: Add a "Ticket Subtask Format" section documenting the required fields and the Test-Driven Development workflow (test first, then code). Use examples from `docs/workflows/ticket.md`.
- test-description: Verify the "Ticket Subtask Format" section exists and contains an example block.
- test-command: `grep "## Ticket Subtask Format" src/plugins/ticket/README.md`
- status: done

## SUBTASK: add usage examples and workflows
- name: add-usage-examples
- description: Add a "Common Workflows" or "Examples" section showing how to use the plugin from start to finish.
- test-description: Verify the examples section exists in the README.
- test-command: `grep "## Examples" src/plugins/ticket/README.md`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done update-ticket-plugin-readme`
- status: done

