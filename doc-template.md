
# Documentation 
use simple markdown like the following. use `shell` blocks for all command line examples and to keep things compact. 

## Folder Structures
Ticket folder structure
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
Ticket api examples
```shell
./dialtone.sh ticket help # print ticket help
./dialtone.sh ticket next # print ticket report and next subtask
./dialtone.sh ticket done # mark the current subtask as done
./dialtone.sh ticket start # create a new ticket
./dialtone.sh ticket list # list all tickets
./dialtone.sh ticket show <ticket> # show a specific ticket
```

## Command Line Workflows
Workflow to create and work on a ticket
```shell
# STEP 1: create a new ticket
./dialtone.sh ticket start fake-ticket

# STEP 2: work on the ticket
./dialtone.sh ticket next

# STEP 3: mark the subtask as done
./dialtone.sh ticket done

# STEP 4: print the ticket report
./dialtone.sh ticket show fake-ticket
```
