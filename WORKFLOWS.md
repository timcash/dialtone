# Workflows and execution model

This document describes how Dialtone turns tickets into repeatable execution using tests, validation, and a task graph.

### Ticket workflow (engineering)
- A ticket is a directory and a description of work.
- A ticket should be small, testable, and written so someone else can execute it.
- Tickets can cover software, hardware, logistics, documentation, and operations.

### Task graph (how tickets execute)
- Ticket execution is treated as a task graph: steps with explicit dependencies.
- Each step is a concrete action (run a test, build an artifact, deploy to a target, verify a measurement, update docs).
- The system advances by selecting the next runnable step, running it, and recording the result.

### Tests and validation
- Tests are written to make changes measurable.
- Validation can be local, simulated, or on-target (a real robot or radio link).
- The goal is repeatability: anyone should be able to reproduce a change and its verification.

### Where to read more
- `docs/workflows/issue_review.md`
- `docs/workflows/ticket_workflow.md`
- `docs/workflows/subtask_expand.md`
- `docs/workflows/www-workflow.md`
- `docs/workflows/www-modernization.md`

