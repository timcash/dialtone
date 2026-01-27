# Branch: standardize-cli-docs
# Tags: documentation, enhancement

# Goal
Standardize CLI documentation and help commands across the project. 
1. Consolidate fragmented command lists in READMEs into single bash blocks with comments.
2. Standardize argument placeholders (e.g., `<issue-id>`, `[<optional>]`, `<list>...`) using POSIX/GNU/Git conventions.

## SUBTASK: Standardize plugin README documentation
- name: standardize-readmes
- description: Consolidate plugin command docs in READMEs into bash code blocks with standardized placeholders.
- test-description: Verify READMEs for test, ticket, github, cloudflare, ide, and ai plugins have consolidated bash blocks.
- test-command: `./dialtone.sh test ticket standardize-cli-docs`
- status: done

## SUBTASK: Standardize CLI help usage strings
- name: standardize-cli-help
- description: Update Go source code for ticket and github plugins to use standardized placeholders in Usage strings.
- test-description: Run `dialtone.sh ticket help` and `dialtone.sh github issue help` to verify new usage strings.
- test-command: `./dialtone.sh test ticket standardize-cli-docs`
- status: done

## SUBTASK: Update all workflows with progress reporting instruction
- name: update-workflows-progress
- description: Update all project workflows (ticket.md, issue_review.md, subtask_expand.md) to include instructions for reporting subtask progress in the standard TDD list format.
- test-description: Verify each workflow file in docs/workflows/ contains the new instruction and the standardized progress list example.
- test-command: `./dialtone.sh test ticket standardize-cli-docs`
- status: done

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: run the cli command `dialtone.sh ticket start standardize-cli-docs`
- test-description: verify ticket is scaffolded and branch created
- test-command: `dialtone.sh test ticket standardize-cli-docs`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket
- test-description: validates all ticket subtasks are done
- test-command: `dialtone.sh ticket done standardize-cli-docs`
- status: todo

## Collaborative Notes
- Context: `[src/plugins/ticket/cli/ticket.go](file:///Users/tim/code/dialtone/src/plugins/ticket/cli/ticket.go)`, `[src/plugins/github/cli/github.go](file:///Users/tim/code/dialtone/src/plugins/github/cli/github.go)`
- Standardized placeholders: `<placeholder>` (required), `[<placeholder>]` (optional), `...` (list).
- Consistently used `<issue-id>` and `<pr-id>` instead of `<id>` or `<number>`.
