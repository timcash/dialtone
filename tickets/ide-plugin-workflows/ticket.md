# Branch: ide-plugin-workflows
# Tags: <labels> (Must match GitHub labels: p0, bug, ready, enhancement, etc.)

# Goal
Provide a CLEAR, HIGH-LEVEL OBJECTIVE. Connect the ticket back to the original GitHub issue context.

## SUBTASK: [Subtask Name]
- name: <name-with-dashes>
- description: Single logic change (< 30 mins). Be precise.
- test-description: Explicitly state how to verify this change.
- test-command: `./dialtone.sh test <path>` or relevant bash command.
- status: todo

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: run the cli command `dialtone.sh ticket start ide-plugin-workflows`
- test-description: verify ticket is scaffolded and branch created
- test-command: `dialtone.sh test ticket ide-plugin-workflows`
- status: todo

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket
- test-description: validates all ticket subtasks are done
- test-command: `dialtone.sh ticket done ide-plugin-workflows`
- status: todo

## Collaborative Notes
- **Context**: Link relevant files (e.g., `[file.go](file:///path/to/file.go)`)
- **Implementation Notes**: Document technical decisions or blockers here.
- **Reference**: https://github.com/timcash/dialtone/issues/<id>

