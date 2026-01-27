# Branch: antigravity-logs-filter
# Tags: <labels> (Must match GitHub labels: p0, bug, ready, enhancement, etc.)

# Goal
Add a `--clean` filtering option to `dialtone ide antigravity logs` that suppresses noisy engine logs and only displays human-relevant information: terminal commands and chat message triggers.


## SUBTASK: Implement `--clean` flag parsing
- name: clean-flag-parsing
- description: Update the `Run` and `runAntigravityLogs` functions in `src/plugins/ide/cli/ide.go` to support an optional `--clean` flag.
- test-description: Verify that the command can be called with `--clean` without erroring.
- test-command: `./dialtone.sh ide antigravity logs --clean` (expected to start but we'll manually check flag propagation)
- status: done

## SUBTASK: Implement line filtering logic
- name: filter-logic
- description: In `src/plugins/ide/cli/ide.go`, replace the simple `tail -f` with a piped process or a Go routine that reads the log line-by-line. If `--clean` is active, only print lines containing `[Terminal]` or `Requesting planner with`.
- test-description: Verify that only "clean" lines are printed to stdout when the flag is used.
- test-command: `./dialtone.sh test ticket antigravity-logs-filter --subtask filter-logic`
- status: done

## SUBTASK: Update IDE plugin README
- name: readme-update
- description: Update `src/plugins/ide/README.md` to document the new `--clean` flag for the `antigravity logs` command.
- test-description: Verify the README has been updated.
- test-command: `grep "\--clean" src/plugins/ide/README.md`
- status: done

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: run the cli command `dialtone.sh ticket start antigravity-logs-filter`
- test-description: verify ticket is scaffolded and branch created
- test-command: `dialtone.sh test ticket antigravity-logs-filter`
- status: done


## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket
- test-description: validates all ticket subtasks are done
- test-command: `dialtone.sh ticket done antigravity-logs-filter`
- status: done

## Collaborative Notes
- **Context**: Link relevant files (e.g., `[file.go](file:///path/to/file.go)`)
- **Implementation Notes**: Document technical decisions or blockers here.
- **Reference**: https://github.com/timcash/dialtone/issues/<id>

