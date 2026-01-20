# Branch: ticket-start-command
# Task: Implement ticket start command in dialtone-dev

# Plugin Structure
1. `./src/plugins/www` - Contains the web dashboard.
2. `./src/plugins/www/README.md` - Contains information for users to understand the plugin at a high level.
3. `./src/plugins/www/app` - Contains a public website for the `dialtone.earth` domain.
4. `./src/plugins/www/cli` - Contains code for the cli command `dialtone-dev www`.
5. `./src/plugins/www/test` - Contains test files for the www plugin.

## Goal
1. Implement a command line plugin for `go run dialtone-dev.go` called `ticket`
2. `go run dialtone-dev.go ticket start <ticket-name> --plugin <plugin-name>` should start a new ticket by:
    1. Creating (or changing to) a new git branch with the branch name from the ticket cli command.
    2. Creating (if it does not exist) a new `ticket/<ticket-name>` folder.
    3. Creating (if it does not exist) a new `ticket/<ticket-name>/task.md` file.
    4. Creating `ticket/<ticket-name>/code` (if it does not exist).
    5. Creating `ticket/<ticket-name>/test` (if it does not exist).
    6. Creating `src/plugin/<plugin-name>` (if it does not exist).
    7. Creating `src/plugin/<plugin-name>/app` (if it does not exist).
    8. Creating `src/plugin/<plugin-name>/cli` (if it does not exist).
    9. Creating `src/plugin/<plugin-name>/test` (if it does not exist).
    10. Creating `src/plugin/<plugin-name>/README.md` (if it does not exist).
3. `go run dialtone-dev.go ticket test <ticket-name>` should test the ticket by:
    1. Running all tests in `ticket/<ticket-name>/test`.
4. `go run dialtone-dev.go ticket done <ticket-name>` should mark the ticket as done by:
    1. verifying all subtasks in `ticket/<ticket-name>/ticket.md` are marked as done.
    2. running all tests via `go run dialtone-dev.go test`.

## Test
- test: Parse Branch metadata from a ticket file.
- verification: Unit test in src/dev_test.go verifying branch extraction.
- test: Create a new git branch automatically and needed folders like `./tickets/<ticket-name>/code`, `./tickets/<ticket-name>/test`, `./tickets/<ticket-name>/task.md`
- verification: `go run dialtone-dev.go ticket start <name>` leads to `git branch` showing the new branch.
- test: Create plugin folders if --plugin flag is used.
- verification: `go run dialtone-dev.go ticket start <name> --plugin <plugin>` creates `src/plugin/<plugin>` structure.
- test: Run ticket tests.
- verification: `go run dialtone-dev.go ticket test <name>` executes tests in `tickets/<name>/test`.
- test: Verify ticket completion.
- verification: `go run dialtone-dev.go ticket done <name>` checks subtasks and runs tests.

## Subtask: Research
- description: Research existing file parsing and git execution in src/dev.go
- status: todo

## Subtask: Plugin Infrastructure
- description: Create the `src/plugins/ticket` directory structure (app, cli, test, README) to house the new plugin code.
- status: todo

## Subtask: Ticket Start - Git & Args
- description: Implement `ticket start` CLI argument parsing (`<ticket-name>`, `--plugin`) and automated git branch creation/switching.
- status: todo

## Subtask: Ticket Start - Ticket Scaffolding
- description: Implement creation of `tickets/<ticket-name>` folder structure (`task.md`, `code/`, `test/`) in the start command.
- status: todo

## Subtask: Ticket Start - Plugin Scaffolding
- description: Implement creation of `src/plugins/<plugin-name>` folder structure (`app/`, `cli/`, `test/`, `README.md`) when `--plugin` is provided.
- status: todo

## Subtask: Ticket Test Command
- description: Implement `ticket test <ticket-name>` to discover and run all tests located in `tickets/<ticket-name>/test`.
- status: todo

## Subtask: Ticket Done Command
- description: Implement `ticket done <ticket-name>` to verify all `task.md` items are checked and run `ticket test` validation.
- status: todo

## Subtask: Verification
- description: Run test: `go run dialtone-dev.go test` to ensure no regressions and verify new commands work as expected.
- status: todo

## Development Cycle
1. Run `go run dialtone-dev ticket start tickets/ticket-start-command/ticket.md` to change the git branch and verify development template files.
2. Update a test before writing new code and run the test to show a failure.
3. Change the system until the test passes.
4. Use `git add` to update git and ensure `.gitignore` is correct.
5. Make a commit with `git commit -m "<message>"`. so you can revert to working tests if needed.

## Development Stages
1. **Ticket**: The first step of any change. Ideal for adding new code that can patch `core` or `plugin` code without changing it directly.
2. **Plugin**: The second step of integrating new code into specific feature areas.
3. **Core**: Core code is reserved for features dealing with networking and deployment (dialtone/dialtone-dev). It is the minimal code required to bootstrap the system.

## Collaborative Notes
[A place for humans and the autocoder to share research, technical decisions, or state between context windows.]

---
Template version: 4.0. To start work: `go run dialtone-dev.go ticket start tickets/ticket-start-command/ticket.md`

---
# Ticket folder layout:
1. `ticket.md` - this file
2. `task.md` - a scratchpad for tracking progress that is not mentioned in `ticket.md`
3. `code/` - all code developed for this change.
4. `test/` - all tests that run to verify this ticket.