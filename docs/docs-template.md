
# Documentation 
use simple markdown like the following. use `shell` blocks for all command line examples and to keep things compact. 

## Folder Structures
Standardized (v2) structure:
```shell
src/tickets/
├── fake-ticket/
│   ├── ticket.md
│   └── test/
│       └── test.go
```

## Command Line Help
Ticket API examples:
```shell
./dialtone.sh ticket help   # print legacy help
./dialtone.sh ticket help   # print v2 help
./dialtone.sh ticket next   # TDD drive for v2
./dialtone.sh ticket done   # Complete v2 ticket
```

# Workflow Example

## STEP 1. Start a new ticket
```shell
# This will create a new ticket directory and switch to the new branch
./dialtone.sh ticket start fake-ticket
```

## STEP 2. Review the ticket
```shell
# Read the ticket.md and any linked documentation or READMEs.
# Plugin Decision: Determine if the feature should be a standalone plugin.
# Identify core dependencies and affected components.
# Outline the initial plan in ticket.md using ## SUBTASK headers.
```

## STEP 3. Ask clarifying questions
```shell
# Verify acceptance criteria are well-defined.
# Check for missing context regarding environment or requirements.
# Action: Use notify_user to ask clarifying questions if blocked.
```


