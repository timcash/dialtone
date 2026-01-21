# Branch: ticket/short-name
# Task: [Ticket Title]

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh plugin create <plugin-name>` to create a new plugin if this ticket is about a plugin
> Run `./dialtone.sh github pull-request` to create a draft pull request

## Goals
1. Use tests files in `ticket/<ticket-name>/test/` to drive all work
2. [item 2]
3. [item 3]

## Non-Goals
1. DO NOT use manual shell commands to verify functionality.
2. DO NOT [item 2]
3. DO NOT [item 3]

## Test
1. all ticket tests are at `ticket/<ticket-name>/test/`
2. all plugin tests are run with `./dialtone.sh plugin test <plugin-name>`
3. all core tests are run with `./dialtone.sh test --core`
4. all tests are run with `./dialtone.sh test`

## Subtask: Research
- description: [List files to explore, documentation to read, or concepts to understand]
- status: todo

## Subtask: Implementation
- description: [NEW/MODIFY] [file_path]: [Short description of change]
- status: todo

## Subtask: Verification
- description: Run test: `./dialtone.sh test`
- status: todo

## Collaborative Notes
[A place for humans and the autocoder to share research, technical decisions, or state between context windows.]

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`
