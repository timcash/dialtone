# How To Verify Prompt (LLM Workflow)

Use this as the prompt for another LLM agent. It follows the ticket workflow style in `src/plugins/ticket/test/integration.go` and the ticket command list in `src/plugins/ticket/README.md`.

## Preconditions
- Branch name and ticket name must be identical.
- All work uses the `./dialtone.sh ticket` system.

## STEP 1: Start the ticket
```shell
# Choose a ticket name (also the branch name).
./dialtone.sh ticket start merge-main-guide
```

Expected:
- `src/tickets/merge-main-guide/` created
- ticket set as current

## STEP 2: Define subtasks in the ticket test
Edit `src/tickets/merge-main-guide/test/test.go` and register subtasks.
```shell
# Example subtask tests (each should return nil only after verified)
dialtest.RegisterTicket("merge-main-guide")
dialtest.AddSubtaskTest("install-tests", RunInstallTests, nil)
dialtest.AddSubtaskTest("build-tests", RunBuildTests, nil)
dialtest.AddSubtaskTest("rebase-build-branch", RunRebaseBuildBranch, nil)
dialtest.AddSubtaskTest("merge-prs", RunMergePRs, nil)
```

## STEP 3: Run the TDD driver
```shell
./dialtone.sh ticket next
```

If blocked by an unanswered question:
```shell
./dialtone.sh ticket ask "Should we merge PR 166 before 167?"
./dialtone.sh ticket next
./dialtone.sh ticket ack "Yes, merge PR 166 first."
```

## STEP 4: Work one subtask at a time
`ticket next` runs the test for the next subtask and can advance repeatedly if tests pass. If you want one-at-a-time control, use `ticket test` or `ticket subtask done`.

```shell
# See the current queue of subtasks
./dialtone.sh ticket subtask list

# Run a single subtask test without advancing status
./dialtone.sh ticket test merge-main-guide --subtask install-tests

# Mark the subtask as done (this command runs the test again)
./dialtone.sh ticket subtask done merge-main-guide install-tests
```

## STEP 5: Implement each subtask with a verifiable test
Each subtask must include a concrete command that proves it is done.

### Subtask: install-tests
```shell
git switch cli-standardization
./dialtone.sh --env test.env install test
```
Pass criteria:
- command exits 0
- install tests report PASS

### Subtask: build-tests
```shell
git switch build-command-tests
./dialtone.sh build test
```
Pass criteria:
- command exits 0
- build tests report PASS

### Subtask: rebase-build-branch
```shell
git fetch origin
git switch build-command-tests
git rebase origin/main
git status -sb
```
Pass criteria:
- rebase completes without conflicts
- working tree clean

### Subtask: merge-prs
```shell
gh pr view 166 --json state,mergedAt
gh pr view 167 --json state,mergedAt
```
Pass criteria:
- both PRs show merged state

## STEP 6: Update ticket summary
```shell
# Write progress to the ticket summary file
cat <<'EOF' > src/tickets/merge-main-guide/agent_summary.md
Ran install tests on cli-standardization: PASS.
Ran build tests on build-command-tests: PASS.
Rebased build-command-tests on origin/main without conflicts.
Merged PRs 166 and 167.
EOF

./dialtone.sh ticket summary update
```

## STEP 7: Validate and finalize
```shell
./dialtone.sh ticket validate
./dialtone.sh ticket test merge-main-guide
./dialtone.sh ticket done
```

## Optional: Subtask JSON upsert example
```shell
cat <<'EOF' > /tmp/merge-main-guide.json
{
  "id": "merge-main-guide",
  "name": "merge-main-guide",
  "description": "Run local tests, rebase build branch, merge PRs 166/167.",
  "subtasks": [
    { "id": "install-tests", "name": "install-tests", "status": "todo" },
    { "id": "build-tests", "name": "build-tests", "status": "todo" },
    { "id": "rebase-build-branch", "name": "rebase-build-branch", "status": "todo" },
    { "id": "merge-prs", "name": "merge-prs", "status": "todo" }
  ]
}
EOF

./dialtone.sh ticket upsert --file /tmp/merge-main-guide.json
./dialtone.sh ticket subtask list
```

## Ticket commands reference
```shell
./dialtone.sh ticket start <name>
./dialtone.sh ticket next
./dialtone.sh ticket test <name> [--subtask <name>]
./dialtone.sh ticket subtask test <ticket-name> <subtask-name>
./dialtone.sh ticket ask "<question>"
./dialtone.sh ticket ack "<answer>"
./dialtone.sh ticket summary update
./dialtone.sh ticket summary
./dialtone.sh ticket subtask list
./dialtone.sh ticket subtask done
./dialtone.sh ticket validate
./dialtone.sh ticket done
```
