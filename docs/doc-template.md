
# Documentation 
use simple markdown like the following. use `shell` blocks for all command line examples and to keep things compact. 

## Folder Structures
Legacy (v1) structure:
```shell
tickets/
├── fake-ticket/
│   ├── ticket.md
│   └── progress.txt
```

Standardized (v2) structure:
```shell
src/tickets_v2/
├── fake-ticket/
│   ├── ticket.md
│   └── test/
│       └── test.go
```

## Command Line Help
Ticket API examples:
```shell
./dialtone.sh ticket help      # print legacy help
./dialtone.sh ticket_v2 help   # print v2 help
./dialtone.sh ticket_v2 next   # TDD drive for v2
./dialtone.sh ticket_v2 done   # Complete v2 ticket
```
