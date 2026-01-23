---
description: Ticket Workflow for dialtone
---

# 1. Start a Ticket
Start a new ticket by running the start command. This creates a new branch and sets up the scaffolding for your work.

```bash
./dialtone.sh ticket start <ticket-name>
```

This command will:
- Create a new branch named `ticket/<ticket-name>`.
- Create a `tickets/<ticket-name>/ticket.md` file if it doesn't exist.
- Create a draft pull request.

# 2. Define Subtasks
Open `tickets/<ticket-name>/ticket.md` and define the work you need to do. Break down the work into small, testable subtasks.
Use the format defined in `docs/rules/rule-ticket-subtask.md`.

```markdown
## SUBTASK: <Task Title>
- description: <What to do>
- test-description: <How to verify it works>
- test-command: <The exact command to run>
- status: todo
```

Example:
```markdown
## SUBTASK: Implement Helper Function
- description: Create a helper function in `src/utils.go` to parse strings.
- test-description: Run the unit test for utils.
- test-command: `./dialtone.sh ticket test <ticket-name>`
- status: todo
```

# 3. Work on a Subtask
Pick the first "todo" subtask and mark it as "progress" in `tickets/<ticket-name>/ticket.md`.

```markdown
- status: progress
```

Focus ONLY on this single subtask. Do not work on multiple things at once.

# 4. Implementation & Testing
Implement the changes required for the subtask.
Run the tests frequently to verify your progress.

```bash
# Run tests for the specific ticket
./dialtone.sh ticket test <ticket-name>

# Or run a specific test command defined in your subtask
./dialtone.sh ticket subtask test <ticket-name> <subtask-name>
```

If you need to define a new test, add it to `tickets/<ticket-name>/test/`.

# 5. Complete Subtask
Once the implementation is complete and the tests pass:
1.  Verify the specific test case defined in the subtask.
2.  Mark the subtask as "done" in `tickets/<ticket-name>/ticket.md`.

```markdown
- status: done
```

# 6. Iterate
Repeat steps 3-5 for each remaining subtask in your list until all work is completed.

# 7. Final Verification & Submission
Once all subtasks are marked as "done", run the final verification to ensure everything is correct and ready for review.

```bash
./dialtone.sh ticket done <ticket-name>
```

This command will:
- Run all tests.
- Verify there are no uncommitted changes and fail if there are.
- Push the branch to GitHub.
- Update the pull request.