# Ticket Plugin
The `ticket` plugin provides a structured, test-driven development (TDD) workflow for the Dialtone project. It manages the lifecycle of a "ticket," which includes git branching, task scaffolding, automated testing, and PR submission.
The `ticket` plugin delegates several commands to the `github` plugin for seamless issue management.

## Core Ticket Commands

```bash
# The primary entry point for new work. Switches branch, scaffolds, and opens PR.
./dialtone.sh ticket start <ticket-name>

# The primary driver for TDD. Validates, runs tests, and manages subtask state.
./dialtone.sh ticket next

# Scaffolds a new local ticket directory. Defaults to current branch name.
# Does not switch branches; ideal for logging side-tasks.
./dialtone.sh ticket add [<ticket-name>]

# Lists local tickets and open remote GitHub issues.
./dialtone.sh ticket list

# Validates the structure and status values of the ticket.md file.
./dialtone.sh ticket validate [<ticket-name>]

# Final step: verifies subtasks, pushes code, and sets PR to ready.
./dialtone.sh ticket done [<ticket-name>]
```


## Subtask Management

Subtasks are defined in the `tickets/<ticket-name>/ticket.md` file. Use the following commands to manage them:

### `ticket subtask` Commands
```bash
# Lists all subtasks and their current status (todo, progress, done, failed).
./dialtone.sh ticket subtask list [<ticket-name>]

# Displays the details of the next pending subtask.
./dialtone.sh ticket subtask next [<ticket-name>]

# Runs the automated test-command defined for the specified subtask.
./dialtone.sh ticket subtask test [<ticket-name>] <subtask-name>

# Updates subtask status in ticket.md to 'done' or 'failed'.
# Enforces git cleanliness.
./dialtone.sh ticket subtask done [<ticket-name>] <subtask-name>
./dialtone.sh ticket subtask failed [<ticket-name>] <subtask-name>
```


## Ticket Subtask Format

A `ticket.md` file is a collection of subtasks. Each subtask must follow this exact markdown format:

```markdown
## SUBTASK: Human readable title
- name: kebab-case-name
- description: Concise paragraph guiding the implementation.
- test-description: How the change should be verified.
- test-command: `dialtone.sh <command-to-run-test>`
- status: todo | progress | done | failed
```

### TDD & Subtask Workflow
The plugin encourages a Test-Driven Development (TDD) approach using `ticket next`:
1. **Plan**: Define small, testable subtasks in `ticket.md`.
2. **Setup Test**: Register your subtask test in `tickets/<ticket-name>/test/test.go`.
3. **Execute Loop**: Run `./dialtone.sh ticket next` to automate the transition from `todo` to `progress`, run tests, and mark as `done`.
4. **Implement**: Write code between loop executions until the subtask passes.

## Examples

### Complete Workflow Example
```bash
# 1. Start a new ticket
./dialtone.sh ticket start feature-xyz

# 2. (Edit tickets/feature-xyz/ticket.md to add subtasks)

# 3. Automated loop: Starts the first 'todo' as 'progress'
./dialtone.sh ticket next

# 4. (Implement code)

# 5. Automated loop: Detects fix, marks as 'done', starts next 'todo'
./dialtone.sh ticket next

# 6. Complete the ticket
./dialtone.sh ticket done
```
