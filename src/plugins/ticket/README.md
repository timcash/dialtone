# Ticket Plugin
The `ticket` plugin provides a structured, test-driven development (TDD) workflow for the Dialtone project. It manages the lifecycle of a "ticket," which includes git branching, task scaffolding, automated testing, and PR submission.
The `ticket` plugin delegates several commands to the `github` plugin for seamless issue management.

## Core Ticket Commands

### `ticket add [ticket-name]`
```bash
# Scaffolds a new local ticket directory. If no name is provided, it uses the current git branch name.
#
# Note: This command does *not switch branches*. It is ideal for agents to log side-tasks or bugs found
# during other work without losing their current context.
./dialtone.sh ticket add bug-found-in-other-module
```

### `ticket start <ticket-name>`
```bash
# The primary entry point for new work. It:
# 1. Creates or switches to a git branch named `<ticket-name>`.
# 2. Scaffolds the ticket directory structure in `tickets/<ticket-name>/`.
# 3. Commits the scaffolding.
# 4. Pushes the branch to the remote repository.
# 5. Opens a draft Pull Request on GitHub.
./dialtone.sh ticket start my-feature-task
```

### `ticket list`
```bash
# Lists all local tickets (directories in `tickets/` containing a `ticket.md`) and open remote GitHub issues.
./dialtone.sh ticket list
```

### `ticket validate [ticket-name]`
```bash
# Validates the structure and status values of the `ticket.md` file.
./dialtone.sh ticket validate my-feature-task
```

### `ticket done [ticket-name]`
```bash
# The final step in the ticket lifecycle. It:
# 1. Verifies all subtasks (except `ticket-done`) are marked as `done`.
# 2. Ensures the git working directory is clean.
# 3. Pushes final changes to the remote.
# 4. Updates the Pull Request to be "ready for review."
# 5. Marks the `ticket-done` subtask as `done`.
./dialtone.sh ticket done my-feature-task
```

## Subtask Management

Subtasks are defined in the `tickets/<ticket-name>/ticket.md` file. Use the following commands to manage them:

### `ticket subtask list [ticket-name]`
```bash
# Lists all subtasks and their current status (`todo`, `progress`, or `done`).
./dialtone.sh ticket subtask list
```

### `ticket subtask next [ticket-name]`
```bash
# Displays the details of the next pending subtask.
./dialtone.sh ticket subtask next
```

### `ticket subtask test [ticket-name] <subtask-name>`
```bash
# Runs the automated `test-command` defined for the specified subtask.
./dialtone.sh ticket subtask test my-subtask
```

### `ticket subtask done [ticket-name] <subtask-name>`
```bash
# Updates the status of the specified subtask to `done` in the `ticket.md` file.
./dialtone.sh ticket subtask done my-subtask
```

## Ticket Subtask Format

A `ticket.md` file is a collection of subtasks. Each subtask must follow this exact markdown format:

```markdown
## SUBTASK: Human readable title
- name: kebab-case-name
- description: Concise paragraph guiding the implementation.
- test-description: How the change should be verified.
- test-command: `dialtone.sh <command-to-run-test>`
- status: todo | progress | done
```

### TDD & Subtask Workflow
The plugin encourages a Test-Driven Development (TDD) approach:
1. **Plan**: Define small, testable subtasks in `ticket.md`.
2. **Setup Test**: Register your subtask test in `tickets/<ticket-name>/test/test.go`.
3. **Verify Failure**: Run `./dialtone.sh ticket subtask test <name>` to ensure the test fails initially.
4. **Implement**: Write the code to fulfill the subtask requirements.
5. **Verify Success**: Run the test again to verify it passes.
6. **Mark Done**: Use `./dialtone.sh ticket subtask done <name>` to track progress.

## Examples

### Complete Workflow Example
```bash
# 1. Start a new ticket
./dialtone.sh ticket start feature-xyz

# 2. (Edit tickets/feature-xyz/ticket.md to add subtasks)

# 3. Check what to do next
./dialtone.sh ticket subtask next

# 4. Run the test for the first subtask
./dialtone.sh ticket subtask test init-logic

# 5. (Implement code)

# 6. Mark it done
./dialtone.sh ticket subtask done init-logic

# 7. Complete the ticket
./dialtone.sh ticket done
```
