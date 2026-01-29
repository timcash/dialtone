# DuckDB Ticket Workflow Demo Commands

This document lists the demo commands to simulate receiving a ticket and improving it, with the reason for each command and the expected output. Each command is shown in a bash code block.

## 1) Start the ticket
Why: Scaffold the ticket and set it as current so subsequent ticket commands target it.

```bash
./dialtone.sh ticket start help-command-plugin
# Expected output (example)
[ticket] Created src/tickets/tickets.duckdb
[ticket] Captured command in src/tickets/tickets.duckdb
[ticket] Ticket help-command-plugin started successfully
```

## 2) Ask a clarifying question
Why: Improve the ticket by capturing an open question before implementation.

```bash
./dialtone.sh ticket ask "Should lighthouse help include examples for help/version?"
# Expected output (example)
[ticket] Captured question in src/tickets/tickets.duckdb
```

## 3) Log a progress update
Why: Record a short status update tied to the ticket’s lifecycle.

```bash
./dialtone.sh ticket log "Drafted initial lighthouse help requirements; need example output and test stub."
# Expected output (example)
[ticket] Captured log in src/tickets/tickets.duckdb
```

## 4) Advance to the next subtask
Why: Drive the TDD loop and run the next subtask’s test.

```bash
./dialtone.sh ticket next
# Expected output (example)
[ticket] Captured command in src/tickets/tickets.duckdb
[ticket] Promoting subtask init to progress
[ticket] Executing test for subtask: init
subtask test not found: init
exit status 1
[ticket] Subtask init failed.

Subtasks for help-command-plugin:
---------------------------------------------------
[prog]      init
---------------------------------------------------
Next Subtask:
Name:            init
Tags:
Dependencies:
Description:     Initialization
Agent-Notes:     exit status 1
Pass-Timestamp:
Fail-Timestamp:  2026-01-28T16:43:34-08:00
Status:          progress
```

## 5) List all subtasks
Why: Review subtask states after a failure to understand what is in progress.

```bash
./dialtone.sh ticket subtask list help-command-plugin
# Expected output (example)
[ticket] Captured command in src/tickets/tickets.duckdb

Subtasks for help-command-plugin:
---------------------------------------------------
[prog]      init
---------------------------------------------------
Next Subtask:
Name:            init
Tags:
Dependencies:
Description:     Initialization
Agent-Notes:     exit status 1
Pass-Timestamp:
Fail-Timestamp:  2026-01-28T16:43:34-08:00
Status:          progress
```

## 6) Validate ticket structure
Why: Ensure the ticket data stored in DuckDB matches the required schema.

```bash
./dialtone.sh ticket validate help-command-plugin
# Expected output (example)
[ticket] Validation complete: help-command-plugin
```

## 7) Test the ticket plugin (optional)
Why: Run the plugin test suite to verify the ticket wiring is correct.

```bash
./dialtone.sh plugin test ticket
# Expected output (example)
PASS: plugin tests completed
```
