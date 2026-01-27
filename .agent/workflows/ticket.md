---
trigger: model_decision
description: ticket workflow for new and existing tickets
---

# Workflow: Ticket-Driven Development (TDD)

This workflow defines the standard process for planning, executing, and managing scope in Dialtone using the `ticket` plugin.

## 1. CLI API Reference

```bash
# Start a new feature or fix (creates branch + PR).
./dialtone.sh ticket start <ticket-name>

# Validate the ticket structure (subtasks, tests, etc).
./dialtone.sh ticket validate <ticket-name>

# View or add a new ticket without switching branches.
./dialtone.sh ticket add <ticket-name>

# Check what to do next.
./dialtone.sh ticket subtask next

# Mark a subtask as done.
./dialtone.sh ticket subtask done <subtask-name>

# Mark a subtask as failed (for investigation/refactor).
./dialtone.sh ticket subtask failed <subtask-name>

# Finalize the ticket (verifies all subtasks and marks PR ready).
./dialtone.sh ticket done
```

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
# This creates tickets/fix-vpn-crash/ without changing your branch
./dialtone.sh ticket add fix-vpn-crash
```

## 6. The TDD Execution Loop

Follow this loop for **every** subtask.

1. **Register the Test**: Define your test in `tickets/<ticket-name>/test/test.go`.
2. **Verify Failure**: Run the test; it must fail first.
   ```bash
   ./dialtone.sh ticket subtask test <name>
   ```
3. **Implement**: Write code to satisfy the test.
4. **Verify Success**: Run the test again to pass.
5. **Mark Done**: 
   ```bash
   ./dialtone.sh ticket subtask done <name>
   ```
6. **Report Progress**: After marking a subtask as done, ALWAYS report the current status of all subtasks to the USER.
   ```bash
   ./dialtone.sh ticket subtask list
   ```
   **Output Example:**
   ```bash
   Subtasks for <ticket-name>:
   ---------------------------------------------------
   [x] subtask-1 (done)
   [/] subtask-2 (progress)
   [ ] subtask-3 (todo)
   [!] subtask-4 (failed)
   ---------------------------------------------------
   ```


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


