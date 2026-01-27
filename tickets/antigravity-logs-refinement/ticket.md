# Branch: antigravity-logs-refinement
# Tags: <labels> (Must match GitHub labels: p0, bug, ready, enhancement, etc.)

# Goal
Refine the `ide antigravity logs` command by implementing an additive flag system (`--chat`, `--commands`), adding minimal line coloring for readability, and renaming the legacy `--clean` flag to `--chat`.

## SUBTASK: Refactor flag parsing and additive logic
- name: refactor-flags
- description: Update `src/plugins/ide/cli/ide.go` to support `--chat` and `--commands`. The logic should be additive: if flags are provided, only show those types. If no flags are provided, show everything.
- test-description: Verify that flags can be combined and logic is additive.
- test-command: `./dialtone.sh ide antigravity logs --chat --commands`
- status: todo

## SUBTASK: Implement log line coloring
- name: log-coloring
- description: Add minimal ANSI color prefixes to filtered lines. Use a small block of color at the start to indicate the category (e.g., Green for Chat, Blue for Commands).
- test-description: Manually verify that output lines have color prefixes.
- test-command: `./dialtone.sh ide antigravity logs --chat`
- status: todo

## SUBTASK: Update documentation and help text
- name: docs-help-update
- description: Update the `printUsage` in `ide.go` and `src/plugins/ide/README.md` to reflect the new flags and behavior.
- test-description: Verify help text shows the new flags.
- test-command: `./dialtone.sh ide help`
- status: todo

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: run the cli command `dialtone.sh ticket start antigravity-logs-refinement`
- test-description: verify ticket is scaffolded and branch created
- test-command: `dialtone.sh test ticket antigravity-logs-refinement`
- status: todo


## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket
- test-description: validates all ticket subtasks are done
- test-command: `dialtone.sh ticket done antigravity-logs-refinement`
- status: todo

## Collaborative Notes
- **Context**: Link relevant files (e.g., `[file.go](file:///path/to/file.go)`)
- **Implementation Notes**: Document technical decisions or blockers here.
- **Reference**: https://github.com/timcash/dialtone/issues/<id>

