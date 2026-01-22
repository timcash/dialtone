# Branch: ticket-short-name (Use - only, no /)
# Task: [Ticket Title]

> [!IMPORTANT]
> See `README.md`, `docs/workflow.md` and `docs/ticket_template.md` for lifecycle docs.
> Run `./dialtone.sh ticket start <ticket-name>` to begin.

## #CAPABILITY: [e.g. Camera, Bus, AI]
List the core systems this ticket interacts with.

## #GOALS
1. Use TDD: Update/Create tests in `tickets/<ticket-name>/test/` first.
2. Follow **Linear Pipeline** style (avoid nested pyramids).
3. Use the `dialtone` logging package for all output.

## #SUBTASK: Research
- description: Explore relevant files and documentation.
- test: Create a failing unit test in `tickets/<ticket-name>/test/unit_test.go`.
- status: todo

## #SUBTASK: Implementation
- description: [MODIFY/NEW] Implement functionality using short, descriptive functions.
- test: Run `./dialtone.sh ticket test <ticket-name>`.
- status: todo

## #SUBTASK: Final Verification
- description: Run full system build and all tests.
- test: Run `./dialtone.sh build --full && ./dialtone.sh test`.
- status: todo

## Collaborative Notes
[Record research, technical decisions, and cross-session state here.]

---
Template version: 7.0. Start work: `./dialtone.sh ticket start <ticket-name>`
