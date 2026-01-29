# Ticket System
Managing asynchronous work through TDD and agent-driven workflows.

## Folder Structures
Standardized (v2) structure:
```shell
src/tickets/
├── tickets.duckdb          # Primary storage (DuckDB)
└── <ticket-id>/
    ├── agent_summary.md    # Deleted after ingestion
    └── test/
        └── test.go         # TDD registry & subtask logic
```

## Command Line Help
Ticket CLI examples:
```shell
./dialtone.sh ticket start <name>    # Initialize a new ticket and git branch
./dialtone.sh ticket next            # Main TDD driver; runs tests and blocks on questions
./dialtone.sh ticket done            # Finalize ticket; requires agent_summary.md

./dialtone.sh ticket summary         # List all agent summaries for current ticket
./dialtone.sh ticket summary update  # Ingest agent_summary.md (deleted on success)
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

## STEP 2. Iterative Development (TDD)
```shell
# Run the TDD drive. It will promote tasks to 'progress' and run tests.
./dialtone.sh ticket next

# If blocked by 10m summary window:
# 1. Update src/tickets/<name>/agent_summary.md
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
- `TICKET_DB_PATH`: Override the default DuckDB storage location. Useful for integration tests to avoid modifying production ticket data.
  ```shell
  TICKET_DB_PATH=src/tickets/test_tickets.duckdb ./dialtone.sh ticket list
  ```

### Ticket Data Model (Go)
```go
type Ticket struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	Status           string    `json:"status"`
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