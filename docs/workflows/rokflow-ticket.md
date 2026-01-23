---
description: Ticket process for code and testing
---

Complete each subtask one at a time

## SUBTASK: Research
- description: Explore relevant files and documentation.
- command: Create a failing unit test in `tickets/<ticket-name>/test/unit_test.go`.

## SUBTASK: Implementation
- description: [MODIFY/NEW] Implement functionality using short, descriptive functions.
- command: Run `./dialtone.sh ticket test <ticket-name>`.

## SUBTASK: Final Verification
- description: Run full system build and all tests.
- command: Run `./dialtone.sh build --full && ./dialtone.sh test`.