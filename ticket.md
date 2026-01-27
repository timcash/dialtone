# Ticket System
- `dialtone` is driven by a core loop of testing and adaptation
- Use tickets and subtasks to document and validate all work
- This process is designed to automate asynchronous work

## golang structure
```go
type TestCondition struct {
	Condition string `json:"condition"`
}

type Subtask struct {
	Name         string `json:"name"`
	Description  string `json:"description"` // LLM markdown friendlydescription of the subtask
	TestCriteria []TestCondition `json:"test_criteria"`
    Status   string `json:"test_status"` // todo, progress, done
}

type Ticket struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
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
			TestCriteria: "echo \"FAIL: Core logic tests\" && exit 1",
			Status:       "todo",
		},
		{
			Name:         "final-polish",
			Description:  "Finalize the implementation and ensure it meets the requirements.",
			TestCriteria: "echo \"FAIL: Final polish tests\" && exit 1",
			Status:       "todo",
		},
	},
}
```




# Ticket Markdown Format
```markdown
# Branch: fake-ticket
# Tags: p0, ready, fake

# Goal
Implement the primary business logic for the fake feature.

## SUBTASK: Core Logic
- name: core-logic
- description: Implement the primary business logic for the fake feature.
- test-description: Verify the core logic is implemented correctly.
- test-condition-1: the binary can build
- test-condition-2: a tcp connection can be made to port $DIALTONE_PORT
- status: todo

## SUBTASK: Final Polish
- name: final-polish
- description: Finalize the implementation and ensure it meets the requirements.
- test-description: Verify the final polish is implemented correctly.
- test-condition-1: the start command prints a metadata report for the user
- test-condition-2: values cpu, network, memory, disk usage appear in the metadata report
- status: todo
```

Running:
```bash
# print ticket report and next subtask
./dialtone.sh ticket next
```
Prints:
```shell
[ticket] Starting next subtask: core-logic
Subtasks for fake-ticket:
---------------------------------------------------
[x] setup-environment 
[/] core-logic (progress)
[ ] final-polish (todo)
---------------------------------------------------
Next Subtask:
Name:        core-logic
Description: Implement the primary business logic for the fake feature.
Test-Criteria:        echo "FAIL: Core logic tests" && exit 1
Status:      progress
```

# Documentation 
use simple markdown like the following

## Folder Structures
```shell
tickets/
├── fake-ticket/
│   ├── ticket.md
│   └── progress.txt
└── test-ticket/
    ├── ticket.md
    └── progress.txt
```

## Command Line Help
how to use the ticket CLI
```bash
./dialtone.sh ticket help # print ticket help
./dialtone.sh ticket next # print ticket report and next subtask
./dialtone.sh ticket done # mark the current subtask as done
./dialtone.sh ticket start # create a new ticket
./dialtone.sh ticket list # list all tickets
./dialtone.sh ticket show <ticket> # show a specific ticket
```

## Command Line Workflows
```bash
# STEP 1: create a new ticket
./dialtone.sh ticket start fake-ticket

# STEP 2: work on the ticket
./dialtone.sh ticket next

# STEP 3: mark the subtask as done
./dialtone.sh ticket done

# STEP 4: print the ticket report
./dialtone.sh ticket show fake-ticket
```
