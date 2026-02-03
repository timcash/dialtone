# Ticket System
Managing asynchronous work through TDD and agent-driven workflows.

## Folder Structures
Standardized (v2) structure:
```shell
src/tickets/
└── <ticket-id>/
    ├── <subtask>-summary.md  # One persistent summary file per subtask
    └── test/
        └── test.go         # TDD registry & subtask logic
    └── <ticket-id>.duckdb  # Per-ticket storage (DuckDB)
```

## Command Line Help
Ticket CLI examples:
```shell
./dialtone.sh ticket start <name>    # Initialize a new ticket and git branch
./dialtone.sh ticket review <name>   # Review mode: planning/readiness questions (no tests/logs/done)
./dialtone.sh ticket next            # Main TDD driver; runs tests and blocks on questions
./dialtone.sh ticket done            # Finalize ticket; requires summaries to be up to date

./dialtone.sh ticket summary         # List all agent summaries for current ticket
./dialtone.sh ticket summary update  # Sync <subtask>-summary.md into DuckDB (no deletion)
./dialtone.sh ticket summary --idle   # Reset 10m timer for idle periods
./dialtone.sh ticket search <query>  # Search through historical agent summaries

./dialtone.sh ticket ask <q>         # Log a question for the user (blocks 'next')
./dialtone.sh ticket ack <m>         # Acknowledge questions/alerts to unblock 'next'
./dialtone.sh ticket log <m>         # Captured developer notes into DuckDB

./dialtone.sh ticket subtask list    # List all subtask states
./dialtone.sh ticket subtask done    # Manually pass a subtask (gated by tests)
./dialtone.sh ticket validate        # Check for status regressions or schema errors
./dialtone.sh ticket upsert --file f # Import ticket definition from JSON

./dialtone.sh ticket key add <n> <v> <p>   # Securely store an encrypted key
./dialtone.sh ticket key <n> <p>            # Lease an encrypted key (outputs value)
./dialtone.sh ticket key list               # List all stored key names
./dialtone.sh ticket key rm <n>             # Remove a key from storage
```

# Workflow Example

## STEP 1. Start a new ticket
```shell
# This will create a new ticket directory and initialize test.go
./dialtone.sh ticket start feature-name
```

## STEP 1b. Review a ticket (prep-only)
```shell
# Review a ticket without starting execution:
# - checks ticket DB/subtasks are well-formed
# - asks readiness questions for the ticket + each subtask
# - does NOT suggest tests/logs or marking subtasks done
./dialtone.sh ticket review feature-name

# Re-run the review iteration (same questions) at any time:
./dialtone.sh ticket next
```

## Ticket state
Tickets have a `state` field in DuckDB:

- `new`: created but not reviewed
- `reviewed`: reviewed and ready to start later
- `started`: execution has begun
- `blocked`: waiting on a question/acknowledgement or missing planning info
- `done`: finalized

## STEP 2. Iterative Development (TDD)
```shell
# Run the TDD drive. It will promote tasks to 'progress' and run tests.
./dialtone.sh ticket next

# If blocked by 10m summary window:
# 1. Update src/tickets/<name>/<subtask>-summary.md
# 2. Run:
./dialtone.sh ticket summary update
```

## STEP 3. Finalize
```shell
# Ensure all subtasks are done and final summary is provided
./dialtone.sh ticket done
```

# Advanced

### Environment Variables
- `TICKET_DB_PATH`: Override the default DuckDB storage location (advanced). By default, each ticket uses its own DuckDB file: `src/tickets/<ticket-id>/<ticket-id>.duckdb`.
  ```shell
  TICKET_DB_PATH=src/tickets/test_tickets.duckdb ./dialtone.sh ticket start my-ticket
  ```

### Ticket Data Model (Go)
```go
type Ticket struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	State            string    `json:"state"`              // new, reviewed, started, blocked, done
	AgentSummary     string    `json:"agent_summary"`      // Consolidated history
	StartTime        string    `json:"start_time"`         // ISO8601
	LastSummaryTime  string    `json:"last_summary_time"`  // ISO8601
	Subtasks         []Subtask `json:"subtasks"`
}
```

### `dialtest` Registry
The ticket test file at `src/tickets/<name>/test/test.go` registers logic for the `ticket next` command:
```go
func init() {
    dialtest.RegisterTicket("my-feature")
    dialtest.AddSubtaskTest("core-logic", CoreLogic, []string{"core"})
}

func CoreLogic() error {
    // Return nil for PASS, error for FAIL
    return nil
}
```