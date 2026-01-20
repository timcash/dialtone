# Branch: ticket-start-command
# Task: Implement ticket start command in dialtone-dev

## Goal
Implement a command that takes a human-written ticket markdown file from the ./tickets folder and bootstraps the developer environment.

## Test
- test: Parse Branch metadata from a ticket file.
- verification: Unit test in src/dev_test.go verifying branch extraction.
- test: Create a new git branch automatically.
- verification: `go run dialtone-dev.go ticket start <name>` leads to `git branch` showing the new branch.
- test: Scaffolding for new plugins.
- verification: If ticket identifies a new plugin, check if app, cli, and tests folders are created.

## Subtask: CLI Implementation
- description: Modify src/dev.go to add the ticket start subcommand and implement logic to read/parse ./tickets/<ticket-name>.md
- status: todo

## Subtask: Git Automation
- description: Execute git checkout -b <branch-name> automatically after parsing
- status: todo

## Subtask: Scaffolding Logic
- description: If the ticket identifies a new plugin, create folders for app, cli, and tests
- status: todo

## Development Cycle
1. Run `go run dialtone-dev ticket start ticket-start-command` to change the git branch and verify development template files.
2. Update a test before writing new code and run the test to show a failure.
3. Change the system until the test passes.
4. Use `git add` to update git and ensure `.gitignore` is correct.

---
Template version: 3.0. To start work: dialtone-dev ticket start ticket-start-command