# Branch: ticket/short-name
# Task: [Ticket Title]

## Goal
[Describe what needs to be accomplished. This helps the human and the autocoder align on the "Why". Provide context here that might not be obvious from the code alone.]

## Test
[List what tests could be run. Centering the ticket system on testing.]
- test: [Description of test]
- verification: [How to run the test]

## Subtask: Research
- description: [List files to explore, documentation to read, or concepts to understand]
- status: todo

## Subtask: Implementation
- description: [NEW/MODIFY] [file_path]: [Short description of change]
- status: todo

## Subtask: Verification
- description: Run test: `go run dialtone-dev.go test`
- status: todo

## Development Cycle
1. Run `go run dialtone-dev ticket start <ticket-name>` to change the git branch and verify development template files.
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
Template version: 4.0. To start work: `go run dialtone-dev.go ticket start <this-file>`

---
# Ticket folder layout:
1. `ticket.md` - this file
2. `task.md` - a scratchpad for tracking progress that is not mentioned in `ticket.md`
3. `code/` - all code developed for this change.
4. `test/` - all tests that run to verify this ticket.
