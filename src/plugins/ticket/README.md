# Ticket System
- `dialtone` is driven by a core loop of testing and adaptation
- Use tickets and subtasks to document and validate all work
- This process is designed to automate asynchronous work


## Ticket Command Line Interface (CLI)
Use these commands to manage all ticket work
```bash
# Scaffolds a new local ticket directory. 
# Does not switch branches.
# Ideal for logging side-tasks while continuing work on current tasks.
./dialtone.sh ticket add [<ticket-name>]

# The primary entry point for new work. Scaffolds and sets current ticket.
./dialtone.sh ticket start <ticket-name>

# Log questions or notes for the current ticket (writes to src/tickets/tickets.duckdb).
./dialtone.sh ticket ask <question>
./dialtone.sh ticket ask --subtask <subtask-name> <question>
./dialtone.sh ticket log <message>

# Tests all subtasks in the ticket
./dialtone.sh plugin test <plugin-name>

# The primary driver for TDD. Validates, runs tests, and manages subtask state.
./dialtone.sh ticket next

# Lists local tickets.
./dialtone.sh ticket list

# Validates the structure and status values stored in DuckDB.
./dialtone.sh ticket validate [<ticket-name>]

# Upsert a ticket definition from JSON (stdin or file).
./dialtone.sh ticket upsert --file path/to/ticket.json
# Or pipe JSON directly:
cat path/to/ticket.json | ./dialtone.sh ticket upsert

# Final step: verifies subtasks and marks ticket complete.
./dialtone.sh ticket done [<ticket-name>]
```


## Subtask Command Line Interface (CLI)
Use these commands to manage all subtask work
```bash
# Lists all subtasks and their current status (todo, progress, done, failed).
./dialtone.sh ticket subtask list [<ticket-name>]

# Print the next incomplete subtask
./dialtone.sh ticket subtask

# Runs the automated test-command defined for the specified subtask.
./dialtone.sh ticket subtask test [<ticket-name>] <subtask-name>

# Updates subtask status in DuckDB to 'done' or 'failed'.
./dialtone.sh ticket subtask done [<ticket-name>] <subtask-name>

# To mark a subtask as failed
./dialtone.sh ticket subtask failed [<ticket-name>] <subtask-name>

# Update subtask agent notes
./dialtone.sh ticket subtask note <ticket-name> <subtask-name> <note>
```


## Ticket Data Model
Tickets and logs are stored in DuckDB at `src/tickets/tickets.duckdb`. The logical structure looks like:
```json
{
  "id": "fake-ticket",
  "tags": ["p0", "ready", "fake"],
  "description": "Implement the primary business logic for the fake feature.",
  "subtasks": [
    {
      "name": "ticket-start",
      "tags": ["setup"],
      "dependencies": [],
      "description": "run the cli command `dialtone.sh ticket start <ticket-name>`",
      "test_conditions": [
        {"condition": "verify ticket is scaffolded"},
        {"condition": "verify branch created"}
      ],
      "agent_notes": "",
      "pass_timestamp": "",
      "fail_timestamp": "",
      "status": "todo"
    }
  ]
}
```


## golang structure
```golang

type Issue struct {
    ID          string    `json:"id"`
    Title       string    `json:"title"`
    Description string    `json:"description"`
    Tags        []string  `json:"tags"`
}

type TestCondition struct {
	Condition string `json:"condition"`
}

type Subtask struct {
	Name           string          `json:"name"`
	Tags           []string        `json:"tags"`
	Dependencies   []string        `json:"dependencies"`
	Description    string          `json:"description"`
	TestConditions []TestCondition `json:"test_conditions"`
	AgentNotes     string          `json:"agent_notes"`
	PassTimestamp  string          `json:"pass_timestamp"`
	FailTimestamp  string          `json:"fail_timestamp"`
	Status         string          `json:"status"` // todo, progress, done, failed
}

type Ticket struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Subtasks    []Subtask `json:"subtasks"`
}



var ExampleTicket = Ticket{
	ID:          "fake-ticket",
	Title:       "Implement Core Logic",
	Description: "Implement the primary business logic for the fake feature.",
	Status:      "todo",
	Subtasks: []Subtask{
		{
			Name:         "core-logic",
			Description:  "Implement the primary business logic for the fake feature.",
			TestCriteria: []TestCondition{{Condition: "the binary can build"}},
			Status:       "todo",
		},
		{
			Name:         "final-polish",
			Description:  "Finalize the implementation and ensure it meets the requirements.",
			TestCriteria: []TestCondition{{Condition: "the start command prints a metadata report"}},
			Status:       "todo",
		},
	},
}
```


# Implementation Command Details

## `./dialtone.sh ticket start <name>`
1. Scaffolds the `src/tickets/<ticket-name>/` directory, creates `src/tickets/<ticket-name>/test/test.go`, and inserts the ticket into `src/tickets/tickets.duckdb`.
2. Sets the current ticket for future `ticket ask/log/next/done` commands.

## `./dialtone.sh ticket next`
1. **Validation**: Loads the ticket from DuckDB and ensures all subtask fields are present.
2. **Dependency Check**: For the next `todo` subtask, verifies that all listed `dependencies` match subtasks with `status: done`.
3. **Test Execution**: Identifies the subtask in `progress`. Dispatches to the `dialtest` registry in `src/tickets/<ticket-name>/test/test.go` to run the specific function registered for that subtask name.
4. **State Transition**:
   - **Pass**: Updates `status: done` in DuckDB and records `pass-timestamp` (current ISO8601).
   - **Fail**: Updates `fail-timestamp` and stays in `progress`, prompting the agent to review `agent-notes`.
5. **Auto-Promotion**: If no task is in `progress`, it marks the first eligible `todo` (dependencies met) as `progress`.

## `./dialtone.sh ticket done`
1. **Final Audit**: Scans DuckDB to ensure all subtasks except `ticket-done` are `done` or `failed`.
2. **Completion**: Marks the ticket complete and logs the action.

## `./dialtone.sh ticket add <ticket-name>`
1. Creates the `src/tickets/<ticket-name>/` directory.
2. Inserts a starter ticket into DuckDB and creates `src/tickets/<ticket-name>/test/test.go`.
3. Sets the current ticket for follow-up `ticket` commands.

# Automated Report Format
Both `ticket next` and `ticket done` output a standardized report to provide the agent with immediate context on the ticket's progress and next steps.

```shell
Subtasks for fake-ticket:
---------------------------------------------------
[fail]      authenticate
[done]      core-logic
[prog]      final-polish
---------------------------------------------------
Next Subtask:
Name:            final-polish
Tags:            documentation
Dependencies:    core-logic, authenticate
Description:     Finalize the implementation and ensure it meets the requirements.
Test-Condition-1: the start command prints a metadata report for the user
Test-Condition-2: values cpu, network, memory, disk usage appear in the metadata report
Agent-Notes:     
Pass-Timestamp:  
Fail-Timestamp:  2026-01-27T16:14:42-08:00
Status:          prog
```


# Ticket Test Folder
The ticket test file is used as an index for all its test commands. It is found at `src/tickets/<ticket-name>/test/test.go` and registers logic for the `ticket next` command to consume.

```golang
import (
	"dialtest"
)

// The init() function handles registration with the core testing library
func init() {
    dialtest.RegisterTicket("fake-ticket")
    
    // register test with the test library and add tags
    dialtest.AddSubtaskTest("setup-environment", SetupEnvironment, []string{"setup", "install"})
    dialtest.AddSubtaskTest("authenticate", Authenticate, []string{"authenticate", "install"})
    dialtest.AddSubtaskTest("core-logic", CoreLogic, []string{"core"})
    dialtest.AddSubtaskTest("final-polish", FinalPolish, []string{"documentation"})
}

func SetupEnvironment() error {
    // Logic to verify environment
    return nil
}

func Authenticate() error {
    // Logic to verify auth
    return nil
}

func CoreLogic() error {
    return nil
}

func FinalPolish() error {
    return nil
}
```

# `dialtest` the dialtone testing library
1. `dialtest.AddSubtaskTest` maps a subtask name (from DuckDB) to a Go function execution.
2. The `ticket next` command uses an internal registry to find these mappings and execute them during the TDD loop.
3. Test functions should return an `error`. A `nil` return signifies a PASS.