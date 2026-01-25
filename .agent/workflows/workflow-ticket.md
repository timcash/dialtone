---
description: Ticket Workflow for dialtone
---

# 1. Verify the ticket exists and is in a valid state
Start a new ticket by running the start command. This creates a new branch and sets up the scaffolding for your work.

```bash
./dialtone.sh ticket start <ticket-name>
```

# 2. Find or Add Subtasks
Open `tickets/<ticket-name>/ticket.md` and look over the subtasks then run `./dialtone.sh ticket subtask next <ticket-name>` to get the next subtask to work on. If the subtask looks to big edit it and break it down into smaller subtasks.

```bash
./dialtone.sh ticket subtask next <ticket-name>
```

# 3. Implementation & Testing
Implement the changes required for the subtask. Run the tests frequently to verify your progress.
If you need to define a new test, add it with the `dialtone.sh test add --tick

```bash
# Run all subtask tests for the specific ticket
./dialtone.sh test ticket <ticket-name>

# Or run a specific subtask test
./dialtone.sh test ticket <ticket-name> --subtask <subtask-name>

# If you worked on a plugin you can run the plugin test
./dialtone.sh test plugin <plugin-name>

# Mark tests with tags so you can use them later as a list of space separated tags
./dialtone.sh test tags tag-one tag-two ...

# Add --list to see the list of tests that would run to any command e.g.
./dialtone.sh test ticket <ticket-name> --list
./dialtone.sh test tags tag-one tag-two ... --list
```



# 4. Complete Subtask
Once the implementation is complete and the tests pass verify the specific test case defined in the subtask. and mark the subtask as done.

```bash
./dialtone.sh test ticket <ticket-name> --subtask <subtask-name>
./dialtone.sh ticket subtask done <ticket-name> <subtask-name>
```


# 5. Iterate
Repeat steps 2-4 for each remaining subtask in your list until all work is completed.
```bash
./dialtone.sh ticket subtask next <ticket-name>
```

# 6. Final Verification & Submission
Once all subtasks are marked as "done", run the final verification to ensure everything is correct and ready for review.

```bash
./dialtone.sh ticket done <ticket-name>
```