# Branch: ticket-short-name (Use - only, no /)
# Tags: [tag1, tag2, tag3]

## SUBTASK: Research
- description: Explore relevant files and documentation.
- test: Create a failing unit test in `tickets/<ticket-name>/test/unit_test.go`.
- status: todo

## SUBTASK: Implementation
- description: [MODIFY/NEW] Implement functionality using short, descriptive functions.
- test: Run `./dialtone.sh ticket test <ticket-name>`.
- status: todo

## SUBTASK: Final Verification
- description: Run full system build and all tests.
- test: Run `./dialtone.sh build --full && ./dialtone.sh test`.
- status: todo
