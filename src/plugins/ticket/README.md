# Ticket Plugin
The `ticket` plugin provides a structured, test-driven development (TDD) workflow for the Dialtone project. It manages the lifecycle of a "ticket," which includes git branching, task scaffolding, automated testing, and PR submission.
The `ticket` plugin delegates several commands to the `github` plugin for seamless issue management.

## Core Ticket Commands

```bash
# Scaffolds a new local ticket directory. Defaults to current branch name.
# Does not switch branches; ideal for logging side-tasks.
./dialtone.sh ticket add [<ticket-name>]

# The primary entry point for new work. Switches branch, scaffolds, and opens PR.
./dialtone.sh ticket start <ticket-name>

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
# Enforces git cleanliness and progress.txt updates.
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
