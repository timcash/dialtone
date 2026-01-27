# Branch: gemini-cli-logs
# Tags: enhancement, ai, logging

# Goal
Capture output from the `gemini` CLI command into the main `dialtone.log` file so that chat history and tool execution details (via debug mode) are preserved and viewable via `dialtone.sh logs` or `dialtone.sh ai opencode ui`.

## SUBTASK: [Subtask Name]
- name: <name-with-dashes>
- description: Single logic change (< 30 mins). Be precise.
- test-description: Explicitly state how to verify this change.
- test-command: `./dialtone.sh test <path>` or relevant bash command.
- status: todo

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: run the cli command `dialtone.sh ticket start gemini-cli-logs`
- test-description: verify ticket is scaffolded and branch created
- test-command: `dialtone.sh test ticket gemini-cli-logs`
- status: done

## SUBTASK: Capture Gemini stdout and stderr
- name: capture-output
- description: Modify `src/plugins/ai/cli/gemini.go` to use `io.Pipe` and `io.MultiWriter` to capture stdout/stderr and send to `logger`.
- test-description: Run a gemini command and grep the log file for the output.
- test-command: `./dialtone.sh ai --gemini "hello" && grep "hello" dialtone.log`
- status: todo

## SUBTASK: Enable verbose tool logging
- name: enable-debug-logs
- description: Update `src/plugins/ai/cli/gemini.go` to pass a `--debug` flag to the underlying `geminicli` if a specific env var or flag is present, or just default to debug if appropriate for capturing tool calls.
- test-description: Run with debug enabled and check for verbose tool output.
- test-command: `./dialtone.sh ai --gemini --debug "hello" && grep "[DEBUG]" dialtone.log`
- status: todo

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket
- test-description: validates all ticket subtasks are done
- test-command: `dialtone.sh ticket done gemini-cli-logs`
- status: todo

## Collaborative Notes
- **Context**: Link relevant files (e.g., `[file.go](file:///path/to/file.go)`)
- **Implementation Notes**: Document technical decisions or blockers here.
- **Reference**: https://github.com/timcash/dialtone/issues/<id>

