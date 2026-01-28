---
trigger: model_decision
description: ticket workflow for new and existing tickets
---

# Workflow: Ticket-Driven Development (TDD)

This workflow defines the standard process for planning, executing, and managing scope in Dialtone.

## 1. START: Initialize Ticket

```bash
# Start a new feature or bugfix ticket
./dialtone.sh ticket start <ticket-name>

# Add a new plugin IF the ticket calls for it
# Verify there is not already a similar plugin
./dialtone.sh plugin add <plugin-name>

# Install dependencies for the new plugin
./dialtone.sh plugin install <plugin-name>
```

## 2. REVIEW: Scope and Context

```bash
# 1. Read the ticket.md and any linked documentation or READMEs.
# 2. Plugin Decision: Determine if the feature should be a standalone plugin.
#    (If so, return to START commands for plugins)
# 3. Identify core dependencies and affected components.
# 4. Outline the initial plan in ticket.md using ## SUBTASK headers.
```

## 3. ASK: Clarify Ambiguities

```bash
# 1. Verify acceptance criteria are well-defined.
# 2. Check for missing context regarding environment or requirements.
# 3. Action: Use notify_user to ask the user clarifying questions if blocked.
./dialtone.sh ticket ask <question>
# OR 
./dialtone.sh ticket ask --subtask <subtask-name> <question>
# 4. Capture general notes for the ticket log.
./dialtone.sh ticket log <message>
```

## 4. SUBTASK SUBLOOP: Iterate Every 10 Minutes

```bash
# Drive the iterative loop: validate, test, and move to next subtask
./dialtone.sh ticket next

# If an unrelated bug or feature is found, add a side-quest ticket
./dialtone.sh ticket add <side-quest-name>
```

### Subtask Checklist
For every iteration, verify:
- **Test Conditions**: Are the `test-condition-*` fields clear and objective?
- **Documentation**: Does this subtask require updates to READMEs or logic documentation?
- **Subtask Size**: If a subtask is taking more than 20 minutes, split it.

    ```markdown
    # Before (Too Large):
    ## SUBTASK: Implement and Test Camera Driver
    - name: camera-driver
    - tags: feature, camera
    - dependencies: setup-environment
    - description: Write the V4L2 logic and add tests.
    - test-condition-1: frame capture returns non-nil buffer
    - agent-notes: needs v4l-utils installed 
    - status: progress

    # After (Split):
    ## SUBTASK: Implement V4L2 device discovery
    - name: camera-discovery
    - tags: camera
    - description: Search /dev for video nodes and return a list of paths.
    - test-condition-1: returns at least one /dev/video* path
    - status: todo

    ## SUBTASK: Implement frame capture logic
    - name: camera-capture
    - tags: camera
    - dependencies: camera-discovery
    - description: Open a video device and read a single buffer.
    - test-condition-1: buffer size matches expected resolution
    - status: todo
    ```

## 5. VALIDATE: Final Verification

```bash
# Run automated ticket validation check
./dialtone.sh ticket validate <ticket-name>

# Build the plugin to ensure no compilation errors
./dialtone.sh plugin build <plugin-name>

# Run all plugin tests
./dialtone.sh plugin test <plugin-name>
```

## 6. CLEANUP: Finalize and Commit

```bash
# 1. Commits: Ensure all changes are committed with clear messages.
# 2. Artifacts: Delete any temporary test files or logs.
# 3. Notify: Summarize accomplishments and ask the user any final questions.

# Close the ticket and mark it as finished
./dialtone.sh ticket done
```

