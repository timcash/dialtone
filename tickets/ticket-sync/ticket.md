# Branch: ticket-sync
# Task: Sync tickets from GitHub

## Goal
Create a CLI command to sync tickets from GitHub to the tickets folder if they don't exist.

## Test
- test: Fetch open issues from GitHub and save as markdown in ./tickets.
- verification: `go run dialtone-dev.go ticket sync` creates new files in ./tickets.

## Subtask: Implementation
- description: Implement sync command in dialtone-dev.go
- status: todo

## Development Cycle
1. Run `go run dialtone-dev ticket start ticket-sync` to change the git branch and verify development template files.
2. Update a test before writing new code and run the test to show a failure.
3. Change the system until the test passes.
4. Use `git add` to update git and ensure `.gitignore` is correct.

---
Template version: 3.0. To start work: dialtone-dev ticket start ticket-sync