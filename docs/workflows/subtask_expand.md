---
trigger: model_decision
description: expand subtasks in the current ticket
---

# Subtask Expansion & Improvement Workflow

This workflow guides LLM agents to **plan and improve** subtasks in the current ticket. You are **NOT executing** the subtasks here—you are **refining the plan** to make them more actionable, testable, and well-structured for other agents to execute.

## Step 1: Identify the Current Ticket

First, determine which ticket you're working with:

```bash
# Get the current ticket name (matches the git branch name)
./dialtone.sh ticket name

# Print the full ticket content to review
./dialtone.sh ticket print
```

The ticket will be located at `tickets/<ticket-name>/ticket.md`.

## Step 2: Review the Ticket Context

Read the ticket file to understand:
- The overall goal and requirements
- Existing subtasks and their current state
- Which subtasks need improvement or expansion

```bash
# Print the full ticket content to review
./dialtone.sh ticket print
```

## Step 3: Identify Subtasks That Need Improvement

First, use `ticket next` to see the current status and identifies the immediate next task:

```bash
# Check the current status and find the next task
./dialtone.sh ticket next
```

Focus on improving a **small set** of subtasks (typically 1-5 subtasks). Look for subtasks that:

1. **Lack clarity**: Vague descriptions that don't guide the LLM effectively
2. **Missing test details**: No clear test-description or test-command
3. **Too large**: Should be broken into smaller, more focused subtasks
4. **Missing context**: Need more description to understand the goal
5. **Status issues**: Subtasks stuck in `progress` that need refinement
6. **Format problems**: Fix anything that doesn't follow the proper subtask format from `docs/workflows/ticket.md`

## Step 4: Understand the Subtask Format

Review the subtask format documentation:

```bash
# Read the ticket workflow to understand subtask format
cat docs/workflows/ticket.md
```

Refer to `docs/workflows/ticket.md` for the complete subtask format. Each subtask must have:

```markdown
## SUBTASK: Small 10 minute task title
- name: name-with-only-lowercase-and-dashes
- description: a single paragraph that guides the LLM to take a small testable step
- test-description: a suggestion that the LLM can use on how to test this change works
- test-command: the actual command to run the test in `dialtone.sh <test-command>` format
- status: one of three status values (todo|progress|done)
```

**Key principles from `docs/workflows/ticket.md`:**
- Each subtask should be a small, ~10 minute task
- Write the test FIRST, then the code (TDD approach)
- Use only `dialtone.sh` and `git` commands when possible
- Use `dialtone.sh ticket next` as your primary TDD execution loop
- Tests are the most important concept in dialtone

## Step 5: Write Strong Subtasks (LLM-Focused)

You are writing instructions for an LLM agent. The subtask should be so clear that the agent can:
1. Identify the exact files to touch,
2. Write the test first,
3. Implement a minimal change,
4. Verify with a single test command.

Use this checklist:

**Clarity**
- Name the exact file(s) or symbol(s) to change
- Define the smallest viable change (avoid multiple goals)
- Specify input/output or behavior in one sentence

**Testability**
- Provide a concrete test scenario the test should assert
- Ensure the test can fail before the fix
- Use one focused `test-command`

**Scope**
- Keep it <= 10 minutes for a single agent
- Avoid design decisions unless the ticket requires one
- If a dependency is missing, split into a new subtask

## Step 6: Use This Subtask Template

```markdown
## SUBTASK: <Short, specific title>
- name: <kebab-case-name>
- description: <single paragraph; include file path(s), function name(s), and exact behavior change>
- test-description: <single sentence describing the test expectation>
- test-command: `dialtone.sh test ticket <ticket-name> --subtask <subtask-name>`
- status: todo
```

## Step 7: Improve Selected Subtasks

Edit `tickets/<ticket-name>/ticket.md` directly to update the improved subtasks. Make sure to:
- Preserve the overall ticket structure
- Keep other subtasks unchanged (unless they also need improvement)
- Maintain proper markdown formatting

## Example: Before and After

**Before (needs improvement):**
```markdown
## SUBTASK: Fix the bug
- name: fix-bug
- description: fix it
- test-description: test it works
- test-command: `dialtone.sh test`
- status: todo
```

**After (improved):**
```markdown
## SUBTASK: Add error handling to camera initialization in camera_linux.go
- name: add-camera-init-error-handling
- description: Update the `InitCamera` function in `src/plugins/camera/app/camera_linux.go` to handle V4L2 device open failures. Return a descriptive error if the device path doesn't exist or cannot be opened. Check for device permissions and provide helpful error messages that guide troubleshooting.
- test-description: Run the test and verify that attempting to initialize a camera with an invalid device path returns a clear error message. The test should check that the error includes the device path and suggests checking permissions or device existence.
- test-command: `dialtone.sh test ticket camera-improvements --subtask add-camera-init-error-handling`
- status: todo
```

## Step 8: Verify Structure

After editing, validate the ticket file so another agent can pick it up cleanly:

```bash
# Validate the ticket structure and status values
./dialtone.sh ticket validate
```

## Step 9: Report Progress
ALWAYS use `ticket next` to report the current status and identify the next task.
```bash
./dialtone.sh ticket next
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


## Notes

- This is a **planning workflow**—you are improving the plan, not executing it
- Focus on a **small set** of subtasks (1-5) per improvement session
- Always refer back to `docs/workflows/ticket.md` for format requirements
- Use command examples in `bash` code blocks to keep formatting consistent
- Ensure subtasks align with the TDD philosophy (test-first approach)
