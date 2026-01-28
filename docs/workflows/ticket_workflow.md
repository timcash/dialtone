description: ticket workflow for new and existing tickets
---

# Workflow: Ticket-Driven Development (TDD)

This workflow defines the standard process for planning, executing, and managing scope in Dialtone.

## 1. CLI API Reference

```bash
./dialtone.sh ticket start <name>           # Start Ticket
./dialtone.sh ticket next                   # Next Subtask
./dialtone.sh ticket add <name>             # Add Side-Task
./dialtone.sh ticket validate <name>        # Validate Ticket
./dialtone.sh ticket subtask failed <name>  # Mark Failed
./dialtone.sh ticket done                   # Finish Ticket
```

## 2. Validation Standard

Before a ticket is considered "Ready", it MUST pass the validation check.

```bash
./dialtone.sh ticket validate <ticket-name>
```

## 3. Ticket Lifecycle

All work starts with a ticket. Use the CLI to manage state.

*Decision needed: If you need a plugin, use the plugin CLI.*
```bash
./dialtone.sh plugin add <plugin-name>     # Add README to src/plugins/<plugin-name>/
./dialtone.sh plugin install <plugin-name> # Install dependencies
./dialtone.sh plugin build <plugin-name>   # Build the plugin
./dialtone.sh plugin test <plugin-name>    # Run tests in src/plugins/<plugin-name>/test/
```

## 4. Splitting Large Subtasks

*Decision needed: If a subtask is taking more than 20 minutes or has multiple distinct steps, split it into smaller subtasks in the `ticket.md`.*

**Before (Too Large):**
```markdown
## SUBTASK: Implement and Test Camera Driver
- name: camera-driver
- tags: feature, camera
- dependencies: setup-environment
- description: Write the V4L2 logic and add tests.
- test-condition-1: frame capture returns non-nil buffer
- agent-notes: needs v4l-utils installed 
- pass-timestamp: 
- fail-timestamp: 
- status: progress
```

**After (Split):**
```markdown
## SUBTASK: Implement V4L2 device discovery
- name: camera-discovery
- tags: camera
- dependencies: setup-environment
- description: Search /dev for video nodes and return a list of paths.
- test-condition-1: returns at least one /dev/video* path
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: todo

## SUBTASK: Implement frame capture logic
- name: camera-capture
- tags: camera
- dependencies: camera-discovery
- description: Open a video device and read a single buffer.
- test-condition-1: buffer size matches expected resolution
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: todo
```

## 5. Handling Side-Quests

*Decision needed: If you find a bug or missing feature unrelated to your current ticket.*
```bash
# IMPORTANT: fill in the ticket.md that gets created with your side-quest task
./dialtone.sh ticket add fix-vpn-crash      # Create scaffold without changing branch
```

## 6. The TDD Execution Loop

Follow this loop for **every** subtask.

1. **Plan**: Define a small, ~10 minute subtask in `ticket.md`.
2. **Register the Test**: Define your test in `src/tickets/<ticket-name>/test/test.go`.
3. **Execute Automated Loop**: ALWAYS use the `next` command to drive the workflow.
   ```bash
   ./dialtone.sh ticket next
   ```
   `ticket next` will:
   - Validate your `ticket.md`.
   - Run tests for your current `progress` subtask.
   - Mark it as `done` if tests pass.
   - Identify and start the next `todo` subtask.
   - Print a progress status chart.


## 7. Recovering from Failures Example

*Decision needed: If a subtask test fails and the fix is complex.*

1. Mark subtask as `failed`.
2. Create new subtasks for investigation and the original goal.
3. Use `ticket subtask list` to verify.

```bash
./dialtone.sh ticket subtask failed <name> # Mark as failed
# Edit ticket.md to add 'fix-dependency' and 'init-video-v2' subtasks
./dialtone.sh ticket subtask next          # Continue with next subtask
```
