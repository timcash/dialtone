# Branch: agent-loop
# Task: Agent Loop

## Goal
The purpose of the agent loop is to detect new tickets and resolve them in a structured way. All system tickets are stored at `./tickets/<ticket_name>.md`.

## Test
- test: Detect a new ticket file in the tickets folder.
- verification: `go run dialtone-dev.go ticket list` shows the new ticket.
- test: Automatic branch creation for a ticket.
- verification: `go run dialtone-dev.go ticket start agent_loop` creates branch `agent-loop`.

## Subtask: Implementation
- description: Implement ticket detection and resolution logic
- status: todo

## Subtask: Verification
- description: Verify that tickets are correctly detected and resolved
- status: todo

## Development Cycle
1. Run `go run dialtone-dev ticket start agent_loop` to change the git branch and verify development template files.
2. Update a test before writing new code and run the test to show a failure.
3. Change the system until the test passes.
4. Use `git add` to update git and ensure `.gitignore` is correct.

---
Template version: 3.0. To start work: dialtone-dev ticket start agent_loop
