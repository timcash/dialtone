# ticket plugin

## `dialtone.sh ticket start <ticket-name>`
1. Switches to git branch `<ticket-name>`, creating it if it doesn't exist.
2. Creates `tickets/<ticket-name>/` directory structure including `test/`.
3. Populates `tickets/<ticket-name>/ticket.md` from `docs/ticket-template.md` (if missing).
4. Creates `tickets/<ticket-name>/progress.txt` for notes (if missing).
5. Generates boilerplate tests in `tickets/<ticket-name>/test/` (if missing).

## `dialtone.sh ticket test <ticket-name>`
1. Runs all tests in `tickets/<ticket-name>/test/` and fails if any fail.

## `dialtone.sh ticket done <ticket-name>`
1. Verify all subtasks in `ticket.md` are marked `done` (excluding `ticket-done`).
2. Runs `dialtone.sh test` to verify all tests in the repo pass.
3. Runs `git status` to verify there are no uncommitted changes.
4. Runs `dialtone.sh github pr` to push the branch to remote and update the pull request.
5. Marks the `ticket-done` subtask as `done`.

## `dialtone.sh ticket subtask list <ticket-name>`
Lists all subtasks defined in `tickets/<ticket-name>/ticket.md` with their status.

## `dialtone.sh ticket subtask next <ticket-name>`
Finds the next subtask (first `todo` or `progress`) and details:
- Name
- Description
- Test command
- Status

## `dialtone.sh ticket subtask test <ticket-name> <subtask-name>`
Executes the `test-command` associated with the specified subtask.
- Automatically prepends `./` to `dialtone.sh` commands if needed.

## `dialtone.sh ticket subtask done <ticket-name> <subtask-name>`
Modifies `tickets/<ticket-name>/ticket.md` to update the subtask status to `done`.
