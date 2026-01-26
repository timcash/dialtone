# Branch: verify-ticket-plugin
# Tags: verification

# Goal
This ticket tracks the verification of all functionality in the `ticket` plugin as documented in `src/plugins/ticket/README.md`.
It also covers the refactoring to move GitHub-specific commands (`create`, `view`, `comment`, `close`) fully to the `github` plugin and remove them from the `ticket` plugin CLI to match the README.

## SUBTASK: Move GitHub Issue Commands to Github Plugin
- name: move-github-commands
- description: Remove `create`, `view`, `comment`, `close` subcommands from `src/plugins/ticket/cli/ticket.go` (or wherever they are registered in ticket plugin). Ensure they are properly registered in `src/plugins/github/cli/github_cli.go` (or equivalent) matching the `github` plugin README.
- test-description: Run `dialtone.sh ticket --help` and verify commands are GONE. Run `dialtone.sh github --help` and verify commands are PRESENT.
- test-command: `dialtone.sh test ticket verify-ticket-plugin`
- status: done

## SUBTASK: Verify and Sync Ticket Help with README
- name: verify-ticket-help-sync
- description: Verify that `dialtone.sh ticket --help` output matches the commands documented in `src/plugins/ticket/README.md`. The README is the source of truth. If there are extra commands in help or missing ones, modify the code to match the README.
- test-description: Run `dialtone.sh ticket --help` and compare with README commands.
- test-command: `dialtone.sh test ticket verify-ticket-plugin`
- status: done

## SUBTASK: Implement Missing GitHub PR Commands
- name: implement-github-pr-commands
- description: The `github` plugin README documents `pr create`, `pr view`, and `pr comment`. However, `src/plugins/github/cli/github.go` does not explicitly handle `view`, `comment`, or `create` as subcommands in `runPullRequest` (it only handles merge, close, and flags). Implement `runPullRequestView`, `runPullRequestComment` and ensure `create` is handled explicitly. Also update `github pr --help`.
- test-description: Write a test `RunGithubPRCommands` that checks `github pr --help` for the new commands and if possible, mocks or validates command dispatch.
- test-command: `dialtone.sh test ticket verify-ticket-plugin --subtask implement-github-pr-commands`
- status: done

## SUBTASK: Test Ticket Add Command
- name: test-ticket-add
- description: Verify that `dialtone.sh ticket add <name>` correctly scaffolds a new local ticket directory with the expected files (`ticket.md`, `test/test.go`, `progress.txt`).
- test-description: Run a test that calls `ticket add`, checks for file existence, and then cleans up.
- test-command: `dialtone.sh test ticket verify-ticket-plugin --subtask test-ticket-add`
- status: done

## SUBTASK: Test Ticket Start Command
- name: test-ticket-start
- description: Verify that `dialtone.sh ticket start <name>` creates a git branch, commits scaffolding, pushes to remote, and attempts to open a PR (or verify the git state changes locally).
- test-description: Validate git branch creation and initial commit structure.
- test-command: `dialtone.sh test ticket verify-ticket-plugin --subtask test-ticket-start`
- status: done

## SUBTASK: Test Ticket List Command
- name: test-ticket-list
- description: Verify that `dialtone.sh ticket list` lists the current local tickets and any open remote GitHub issues.
- test-description: Create a dummy ticket and verify it appears in the output of `ticket list`.
- test-command: `dialtone.sh test ticket verify-ticket-plugin --subtask test-ticket-list`
- status: done

## SUBTASK: Test Ticket Validate Command
- name: test-ticket-validate
- description: Verify that `dialtone.sh ticket validate <name>` correctly checks the structure and status values of the `ticket.md` file.
- test-description: Create a valid and an invalid `ticket.md` and verify the validation command returns success and failure respectively.
- test-command: `dialtone.sh test ticket verify-ticket-plugin --subtask test-ticket-validate`
- status: done

## SUBTASK: Test Ticket Subtask List Command
- name: test-ticket-subtask-list
- description: Verify that `dialtone.sh ticket subtask list` correctly lists all subtasks and their current statuses.
- test-description: Parse the output of `subtask list` for a known `ticket.md` and match against expected subtasks.
- test-command: `dialtone.sh test ticket verify-ticket-plugin --subtask test-ticket-subtask-list`
- status: done

## SUBTASK: Test Ticket Subtask Next Command
- name: test-ticket-subtask-next
- description: Verify that `dialtone.sh ticket subtask next` displays the details of the next pending subtask (first non-done item).
- test-description: Set up a `ticket.md` with mixed statuses and verify `next` returns the correct subtask.
- test-command: `dialtone.sh test ticket verify-ticket-plugin --subtask test-ticket-subtask-next`
- status: done

## SUBTASK: Test Ticket Subtask Test Command
- name: test-ticket-subtask-test
- description: Verify that `dialtone.sh ticket subtask test [ticket-name] <subtask-name>` runs the specific test command defined in the subtask.
- test-description: Define a subtask with a simple echo test command and verify it executes.
- test-command: `dialtone.sh test ticket verify-ticket-plugin --subtask test-ticket-subtask-test`
- status: done

## SUBTASK: Test Ticket Subtask Done Command
- name: test-ticket-subtask-done
- description: Verify that `dialtone.sh ticket subtask done [ticket-name] <subtask-name>` updates the status of the subtask to `done` in `ticket.md`.
- test-description: Create a dummy ticket with a todo subtask, run `done` command, and verify status is updated.
- test-command: `dialtone.sh test ticket verify-ticket-plugin --subtask test-ticket-subtask-done`
- status: done

## SUBTASK: Test Ticket Subtask Failed Command
- name: test-ticket-subtask-failed
- description: Verify that `dialtone.sh ticket subtask failed [ticket-name] <subtask-name>` updates the status of the subtask to `failed` in `ticket.md`.
- test-description: Run the command and check the file content for the status change.
- test-command: `dialtone.sh test ticket verify-ticket-plugin --subtask test-ticket-subtask-failed`
- status: done

## SUBTASK: Test Ticket Done Command
- name: test-ticket-done
- description: Verify that `dialtone.sh ticket done <ticket-name>` performs the final verification, git check, and PR update.
- test-description: Mock the prerequisites (all subtasks done) and verify the command execution flow.
- test-command: `dialtone.sh test ticket verify-ticket-plugin --subtask test-ticket-done`
- status: done
