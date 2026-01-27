# Branch: issue-ticket-workflow
# Tags: p0, workflow, documentation

# Goal
Refactor the issue review workflow and GitHub plugin to support a streamlined triage process. This includes moving the workflow documentation to a dedicated ticket and implementing a `--ready` shortcut to mark issues as "Ticket" ready.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start issue-ticket-workflow`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket issue-ticket-workflow`
- status: done

## SUBTASK: Implement github issue <id> --ready
- name: implement-ready-flag
- description: Modify the GitHub plugin to support `github issue <id> --ready` which adds the `ticket` label to the specified issue.
- test-description: Run the command on a test issue and verify the label is added.
- test-command: `./dialtone.sh github issue 97 --ready`
- status: done

## SUBTASK: Update workflow documentation
- name: update-workflow-docs
- description: Update `docs/workflows/issue_review.md` to use the new `--ready` shortcut.
- test-description: Verify the documentation content is correct.
- test-command: `grep "\-\-ready" docs/workflows/issue_review.md`
- status: done

## SUBTASK: Update GitHub plugin README
- name: update-github-readme
- description: Add the new issue shortcut command to `src/plugins/github/README.md`.
- test-description: Verify the README has the updated usage.
- test-command: `grep "\-\-ready" src/plugins/github/README.md`
- status: done
## SUBTASK: Implement full suite of label shortcuts
- name: implement-label-shortcuts
- description: Add flags like `--p0`, `--bug`, `--docs`, etc., to the GitHub plugin for fast triage.
- test-description: Verify multiple labels can be added at once.
- test-command: `./dialtone.sh github issue 97 --bug --p1`
- status: done

## SUBTASK: Create Labeling Reference in workflow docs
- name: document-labeling-reference
- description: Add a "Labeling Reference" table to `docs/workflows/issue_review.md` in a clean, agent-readable format.
- test-description: Verify the table exists and is correctly formatted.
- test-command: `grep "## 5. Labeling Reference" docs/workflows/issue_review.md`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review.
- test-description: validates all ticket subtasks are done
- test-command: `dialtone.sh ticket done issue-ticket-workflow`
- status: done

## Collaborative Notes
- Moved work from `integrate-bubbletea-tui` to this ticket to maintain focus.
- The `--ready` flag is a thin wrapper over `issue edit --add-label ticket`.

