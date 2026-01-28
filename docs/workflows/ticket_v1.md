---
trigger: model_decision
description: ticket workflow for new and existing tickets (v1 specific)
---

# Workflow: Ticket-Driven Development (TDD) (v1)

This workflow defines the standard process for planning, executing, and managing scope in Dialtone (Legacy v1).

## 1. CLI API Reference

| Action | Legacy CLI (v1) |
| :--- | :--- |
| **Start Ticket** | `./dialtone.sh ticket start <name>` |
| **Next Subtask** | `./dialtone.sh ticket next` |
| **Add Side-Task** | `./dialtone.sh ticket add <name>` |
| **Validate Ticket** | `./dialtone.sh ticket validate <name>` |
| **Mark Failed** | `./dialtone.sh ticket subtask failed <name>` |
| **Finish Ticket** | `./dialtone.sh ticket done` |

## 2. Validation Standard

Before a ticket is considered "Ready", it MUST pass the validation check.

```bash
./dialtone.sh ticket validate <ticket-name>
```

The validator ensures:
1. The `# Goal` section is present.
2. Every `## SUBTASK` has a unique `name`, `description`, `test-command`, and `status`.
3. The `status` is one of: `todo`, `progress`, `done`, `failed`.

## 3. Ticket Lifecycle

All work starts with a ticket. Use the CLI to manage the state of your work.

*Decision needed: If you need to create a plugin, use the plugin CLI.*
```bash
./dialtone.sh plugin add <plugin-name> # Add a README.md to src/plugins/<plugin-name>/README.md
./dialtone.sh plugin install <plugin-name> # Install dependencies
./dialtone.sh plugin build <plugin-name> # Build the plugin
./dialtone.sh test plugin <plugin-name> # Runs tests in src/plugins/<plugin-name>/test/
```

## 4. Splitting Large Subtasks

*Decision needed: If a subtask is taking more than 20 minutes or has multiple distinct steps, split it into smaller subtasks in the `ticket.md`.*

**Before (Too Large):**
```markdown
## SUBTASK: Implement and Test Camera Driver
- name: camera-driver
- description: Write the V4L2 logic and add tests.
- status: progress
```

**After (Split):**
```markdown
## SUBTASK: Implement V4L2 device discovery
- name: camera-discovery
- description: Search /dev for video nodes and return a list of paths.
- test-command: `dialtone.sh test ticket <name> --subtask camera-discovery`
- status: todo

## SUBTASK: Implement frame capture logic
- name: camera-capture
- description: Open a video device and read a single buffer.
- test-command: `dialtone.sh test ticket <name> --subtask camera-capture`
- status: todo
```

## 5. Handling Side-Quests

*Decision needed: If you find a bug or a missing feature unrelated to your current ticket.*
```bash
# You found a bug in 'vpn' while working on 'camera'
# This creates the scaffold without changing your branch
./dialtone.sh ticket add fix-vpn-crash
```

## 6. The TDD Execution Loop

Follow this loop for **every** subtask.

1. **Plan**: Define a small, ~10 minute subtask in `ticket.md`.
2. **Register the Test**: Define your test in `tickets/<ticket-name>/test/test.go`.
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


## 7. Recovering from Failures

*Decision needed: If a subtask test fails and the fix is complex, do not keep the subtask in `progress` indefinitely.*

1. Mark the current subtask as `failed` (using `./dialtone.sh ticket subtask failed <name>`).
2. Create two new subtasks in `ticket.md`: one for the **investigation/refactoring** and one for the **original goal**.
3. Use `ticket subtask list` to verify the new plan.

```bash
# If subtask 'init-video' is blocked by a dependency bug:
./dialtone.sh ticket subtask failed init-video
# (Edit ticket.md to add 'fix-dependency' and 'init-video-v2' subtasks)
./dialtone.sh ticket subtask next
```
