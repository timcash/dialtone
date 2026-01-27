# Branch: gemini-sessions
# Tags: ai, feature

# Goal
Enable session management for the Gemini CLI integration, allowing users to list, resume, and delete chat sessions using standard flags.

## SUBTASK: [Subtask Name]
- name: <name-with-dashes>
- description: Single logic change (< 30 mins). Be precise.
- test-description: Explicitly state how to verify this change.
- test-command: `./dialtone.sh test <path>` or relevant bash command.
- status: todo

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: run the cli command `dialtone.sh ticket start gemini-sessions`
- test-description: verify ticket is scaffolded and branch created
- test-command: `dialtone.sh test ticket gemini-sessions`
- status: done

## SUBTASK: Update gemini.go to support session flags
- name: impl-session-flags
- description: Modify `src/plugins/ai/cli/gemini.go` to detect session flags (`--list-sessions`, `--resume`, `--delete-session`) and avoid forcing the `chat` subcommand or requiring a prompt.
- test-description: Verify `--list-sessions` works.
- test-command: `./dialtone.sh ai --gemini --list-sessions`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket
- test-description: validates all ticket subtasks are done
- test-command: `dialtone.sh ticket done gemini-sessions`
- status: todo

## Collaborative Notes
- **Context**: Link relevant files (e.g., `[file.go](file:///path/to/file.go)`)
- **Implementation Notes**: Document technical decisions or blockers here.
- **Reference**: https://github.com/timcash/dialtone/issues/<id>

