# Branch: gemini-cli-logs
# Tags: enhancement, ai, logging

# Goal
Capture output from the `gemini` CLI command into the main `dialtone.log` file, and provide a colorized terminal experience for tool calls and errors via `--errors` and `--tools` flags in raw mode.



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
- test-command: `./dialtone.sh ai gemini "hello" && grep "hello" dialtone.log`
- status: done

## SUBTASK: Refactor CLI syntax to use positional 'gemini'
- name: cli-syntax-refactor
- description: Change `--gemini` flag to `gemini` positional argument in `src/plugins/ai/cli/ai.go`.
- test-description: Verify both `ai gemini --help` and `ai gemini "hello"` work.
- test-command: `./dialtone.sh ai gemini --help`
- status: done

## SUBTASK: Enable verbose tool logging
- name: enable-debug-logs
- description: Update `src/plugins/ai/cli/gemini.go` to pass a `--debug` flag to the underlying `geminicli` if a specific env var or flag is present, or just default to debug if appropriate for capturing tool calls.
- test-description: Run with debug enabled and check for verbose tool output.
- test-command: `./dialtone.sh ai gemini --debug "hello" && grep "[DEBUG]" dialtone.log`
- status: done

## SUBTASK: Implement raw logging mode
- name: suppress-logger-prefix
- description: Add an option to `logger.go` to suppress the prefix (timestamp/level/details) and use it in `gemini.go` for cleaner terminal output.
- test-description: Run gemini with a flag that minimizes prefixing in terminal output.
- test-command: `./dialtone.sh ai gemini "hello" --raw`
- status: done

## SUBTASK: Add color support to logger
- name: add-logger-colors
- description: Add ANSI color constants (Red, Green, Yellow, Cyan, Reset) to `src/core/logger/logger.go` to be used by plugins for terminal highlighting.
- test-description: Verify constants are available and usable in a simple test.
- test-command: `go test ./src/core/logger/...` (or similar manual verification)
- status: todo

## SUBTASK: Implement colorized markers and filter flags in gemini.go
- name: gemini-colorized-filters
- description: Update `src/plugins/ai/cli/gemini.go` to parse `--errors` and `--tools` flags. When these flags are active (especially in `--raw` mode), tool calls should be marked with a color (e.g., Green/Cyan) and errors with Red at the start of the line.
- test-description: Run `ai gemini "hello" --raw --tools` and verify tool calls (if any) are colorized.
- test-command: `./dialtone.sh ai gemini "hello" --raw --tools --errors`
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

