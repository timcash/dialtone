```signature
status: wait
note: waiting for agent
updated_at: pending
```

# Task: Agent Workflow Test

Use this file as the contract for how you work.

## Required Workflow
1. Immediately update the signature block:
   `status: work`
2. Open only this folder:
   `src/plugins/worktree/src_v1/agent_test`
3. Inspect files:
   `calc.go` and `test.go`
4. Fix the bug in `calc.go`.
5. Verify by running:
   `go run test.go calc.go`
6. If the command prints `PASS: Add(2,2)=4`, update signature to:
   `status: done`
7. If verification fails or you are blocked, update signature to:
   `status: fail`
8. Set `note:` with a short summary and set `updated_at:` timestamp.
9. Stop after updating the signature.
