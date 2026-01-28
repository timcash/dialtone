# Ticket System v2
- `dialtone` is driven by a core loop of testing and adaptation
- Use tickets and subtasks to document and validate all work
- This process is designed to automate asynchronous work


## Ticket Command Line Interface (CLI)
Use these commands to manage all ticket work
```bash
# Scaffolds a new local ticket directory. 
# Does not switch branches.
# Ideal for logging side-tasks while continuing work on current tasks.
./dialtone.sh ticket_v2 add [<ticket-name>]

# The primary entry point for new work. Switches branch, scaffolds, and opens PR.
./dialtone.sh ticket_v2 start <ticket-name>

# Tests all subtasks in the ticket
./dialtone.sh ticket_v2 test [<ticket-name>]

# The primary driver for TDD. Validates, runs tests, and manages subtask state.
./dialtone.sh ticket_v2 next

# Lists local tickets and open remote GitHub issues.
./dialtone.sh ticket_v2 list

# Validates the structure and status values of the ticket.md file.
./dialtone.sh ticket_v2 validate [<ticket-name>]

# Final step: verifies subtasks, pushes code, and sets PR to ready.
./dialtone.sh ticket_v2 done [<ticket-name>]
```


## Subtask Command Line Interface (CLI)
Use these commands to manage all subtask work
```bash
# Lists all subtasks and their current status (todo, progress, done, failed).
./dialtone.sh ticket_v2 subtask list [<ticket-name>]

# Print the next incomplete subtask
./dialtone.sh ticket_v2 subtask

# Runs the automated test-command defined for the specified subtask.
./dialtone.sh ticket_v2 subtask test [<ticket-name>] <subtask-name>

# Updates subtask status in ticket.md to 'done' or 'failed'.
# Enforces git cleanliness.
./dialtone.sh ticket_v2 subtask done [<ticket-name>] <subtask-name>
# To mark a subtask as failed
./dialtone.sh ticket_v2 subtask failed [<ticket-name>] <subtask-name>
```


## Ticket_v2 Markdown Format
Use this format whenever you create a new ticket. This structure is the source of truth for the automated state machine.
```markdown
# Name: fake-ticket
# Tags: p0, ready, fake

# Goal
Implement the primary business logic for the fake feature.

## SUBTASK: Authenticate
- name: authenticate
- tags: setup, install
- dependencies: setup-environment
- description: Allow the user to log in via CLI commmands
- test-condition-1: look for an api key
- test-condition-2: print a link if no api key is found
- agent-notes: Could not find documentation for authentication
- pass-timestamp: 
- fail-timestamp: 2026-01-27T16:14:42-08:00
- status: failed

## SUBTASK: Core Logic
- name: core-logic
- tags: core
- dependencies: setup-environment
- description: Implement the primary business logic for the fake feature.
- test-condition-1: the binary can build
- test-condition-2: a tcp connection can be made to port $DIALTONE_PORT
- agent-notes:
- pass-timestamp: 2026-01-27T18:28:42-08:00
- fail-timestamp: 2026-01-27T16:14:42-08:00
- status: done

## SUBTASK: Final Polish
- name: final-polish
- tags: documentation
- dependencies: core-logic, authenticate, setup-environment
- description: Finalize the implementation and ensure it meets the requirements.
- test-condition-1: the start command prints a metadata report for the user
- test-condition-2: values cpu, network, memory, disk usage appear in the metadata report
- agent-notes:
- pass-timestamp: 
- fail-timestamp: 2026-01-27T16:14:42-08:00
- status: todo
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

## `./dialtone.sh ticket_v2 start <name>`
1. Checks if a branch named `<name>` exists via `git branch --list`.
2. Creates and switches to the branch if it doesn't exist (`git checkout -b <name>`).
3. Scaffolds the `src/tickets_v2/<ticket-name>/` directory with `ticket.md` (populated from a template) and `src/tickets_v2/<ticket-name>/test/test.go`.
4. Performs an initial commit: `git add . && git commit -m "chore: start ticket <name>"`.
5. Pushes the branch: `git push -u origin <name>`.
6. Creates a **Draft Pull Request** on GitHub using the `gh pr create` CLI or internal GitHub plugin.

## `./dialtone.sh ticket_v2 next`
1. **Validation**: Parses `ticket.md` using a regex or markdown parser to ensure all `SUBTASK` fields are present.
   - **Regression Check**: If a subtask has a `fail-timestamp` that is newer than its `pass-timestamp`, the command fails, indicating a regression (a previously passing test is now failing).
2. **Dependency Check**: For the next `todo` subtask, verifies that all listed `dependencies` match subtasks with `status: done`.
3. **Test Execution**: Identifies the subtask in `progress`. Dispatches to the `dialtest` registry in `src/tickets_v2/<ticket-name>/test/test.go` to run the specific function registered for that subtask name.
4. **State Transition**:
   - **Pass**: Updates `status: done` in `ticket.md`, records `pass-timestamp` (current ISO8601), and auto-commits the change.
   - **Fail**: Updates `fail-timestamp` and stays in `progress`, prompting the agent to review `agent-notes`.
5. **Auto-Promotion**: If no task is in `progress`, it marks the first eligible `todo` (dependencies met) as `progress`.

## `./dialtone.sh ticket_v2 done`
1. **Final Audit**: Scans `ticket.md` to ensure all subtasks except `ticket-done` are `done` or `failed`.
2. **Git Hygiene**: Verifies `git status` is clean. Performs a final `git push`.
3. **PR Finalization**: Updates the GitHub Pull Request status from "Draft" to "Ready for Review".
4. **Context Reset**: Switches the local git branch back to `main`.

## `./dialtone.sh ticket_v2 add <ticket-name>`
1. Creates the `src/tickets_v2/<ticket-name>/` directory without changing the current git branch.
2. Scaffolds the basic `ticket.md` and `src/tickets_v2/<ticket-name>/test/test.go` files.
3. This is a "side-car" command to capture ideas or bugs without interrupting the primary feature flow.

# Automated Report Format
Both `ticket_v2 next` and `ticket_v2 done` output a standardized report to provide the agent with immediate context on the ticket's progress and next steps.

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
The ticket test file is used as an index for all its test commands. It is found at `src/tickets_v2/<ticket-name>/test/test.go` and registers logic for the `ticket_v2 next` command to consume.

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
1. `dialtest.AddSubtaskTest` maps a subtask name (from `ticket.md`) to a Go function execution.
2. The `ticket_v2 next` command uses an internal registry to find these mappings and execute them during the TDD loop.
3. Test functions should return an `error`. A `nil` return signifies a PASS.