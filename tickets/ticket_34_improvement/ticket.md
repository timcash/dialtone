# Branch: ticket-34-improvement
# Task: Design Prototype Integration of Code and Dialtone CLI

## Goal
Automate the manual developer loop described in AGENT.md. The goal is to allow dialtone-dev to autonomously identify, solve, and submit improvements to the codebase.

## Test
- test: Developer command fetches open tickets.
- verification: `go run dialtone-dev.go developer` logs show fetching from GitHub.
- test: Subagent is launched with correct task file.
- verification: Check process list or logs for `dialtone-dev subagent --task ...`.

## Subtask: Skeleton Implementation
- description: Implement the developer command skeleton in src/dev.go
- status: todo

## Subtask: Subagent Interface
- description: Define the subagent interface (wrapping opencode or similar)
- status: todo

## Development Cycle
1. Run `go run dialtone-dev ticket start ticket_34_improvement` to change the git branch and verify development template files.
2. Update a test before writing new code and run the test to show a failure.
3. Change the system until the test passes.
4. Use `git add` to update git and ensure `.gitignore` is correct.

---
Template version: 3.0. To start work: dialtone-dev ticket start ticket_34_improvement
