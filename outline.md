# Documentation Reconciliation Summary: Ticket v1 & v2

## 1. Core Repository Overview (README.md)
   - **Project Structure**: Integrated `src/tickets_v2/` as the new standard while labeling `tickets/` as Legacy.
   - **Ticket Lifecycle**: Documented side-by-side lifecycles for both systems.
   - **Testing Interface**: Added `test ticket_v2` alongside `test ticket` for both full and subtask-specific testing.
   - **Directory Layouts**: Detailed the differences between v1 (`task.md`) and v2 (`test/test.go`) internal structures.
   - **Data Objects**: Updated the `TICKET` definition to reflect dual directory and lifecycle paths.

## 2. Planning & Execution Workflows (docs/workflows/)
   - **Ticket Workflow**: Created a unified API mapping table for both v1 and v2 command sets.
   - **Subtask Expansion**: Standardized identification, printing, and refinement steps for both `tickets/` and `src/tickets_v2/` paths.
   - **Issue Review**: Updated promotion (IMPROVE) logic to allow issues to be funneled into either ticket system based on complexity.

## 3. Agent & Developer Guidance (.agent/)
   - **CLI Rules**: Updated `rule-cli.md` to ensure agents choose the appropriate tool based on the ticket version.
   - **Documentation Templates**: (Modified `doc-template.md`) Standardized the foundation for new documentation to show both v1 and v2 layouts.

## 4. Verification & Integration Status
   - **Specification Compliance**: Verified `ticket_v2` against 100% of the granular specification in `ticket_v2.md` (now deleted).
   - **Environment Safety**: Confirmed the integration suite reliably restores the starting git context (branch restoration via `defer`).
   - **Git Hygiene**: Ensured both documentation and code enforce mandatory cleanliness checks (`git status`) before state transitions.
   - **Registry Integration**: Confirmed plugin registration in the main `dialtone.sh` CLI wrapper.
