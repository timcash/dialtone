# Branch: <branch-name>
# Tags: <labels> (Must match GitHub labels: p0, bug, ready, enhancement, etc.)

# Goal
Provide a CLEAR, HIGH-LEVEL OBJECTIVE. Connect the ticket back to the original GitHub issue context.

## SUBTASK: [Subtask Name]
- name: <name-with-dashes>
- tags: <tags>
- dependencies: <dependencies>
- description: Single logic change (< 30 mins). Be precise.
- test-condition-1: <test-condition-1>
- test-condition-2: <test-condition-2>
- agent-notes: <agent-notes>
- pass-timestamp: 
- fail-timestamp: 
- status: todo

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- tags: setup
- dependencies: 
- description: run the cli command `dialtone.sh ticket start <ticket-name>`
- test-condition-1: verify ticket is scaffolded
- test-condition-2: verify branch created
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: todo

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- tags: cleanup
- dependencies: <last-subtask-name>
- description: run the ticket cli to verify all steps to complete the ticket
- test-condition-1: validates all ticket subtasks are done
- test-condition-2: verifies git status is clean
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: todo

## Collaborative Notes
- **Context**: Link relevant files (e.g., `[file.go](file:///path/to/file.go)`)
- **Implementation Notes**: Document technical decisions or blockers here.
- **Reference**: https://github.com/timcash/dialtone/issues/<id>

