# Verification Audit: ticket

This document audits the `ticket` implementation against the specifications in `ticket.md`.

## Command Verification

| Command | Requirement | Test Case | Status |
|---------|-------------|-----------|--------|
| `add` | Scaffolds directory and files | `TestAddGranular` | [x] |
| `add` | Does not switch branches | `TestAddGranular` | [x] |
| `start` | Checks if branch exists | `TestStartGranular` | [x] |
| `start` | Creates/Switches branch | `TestStartGranular` | [x] |
| `start` | Performs initial commit | `TestStartGranular` | [x] |
| `start` | Pushes branch with `-u` | `TestStartGranular` | [x] |
| `start` | Creates Draft PR | `TestStartGranular` | [x] |
| `next` | Validates `ticket.md` structure | `TestNextGranular` | [x] |
| `next` | Verifies dependencies | `TestNextGranular` | [x] |
| `next` | Records `pass-timestamp` | `TestNextGranular` | [x] |
| `next` | Records `fail-timestamp` | `TestNextGranular` | [x] |
| `next` | Performs auto-commit on pass | `TestNextGranular` | [x] |
| `next` | Auto-promotes `todo` -> `prog` | `TestNextGranular` | [x] |
| `done` | Final audit (all done/fail) | `TestDoneGranular` | [x] |
| `done` | Verifies `git status` clean | `TestDoneGranular` | [x] |
| `done` | Performs final push | `TestDoneGranular` | [x] |
| `done` | Marks PR as Ready | `TestDoneGranular` | [x] |
| `done` | Switches back to `main` | `TestDoneGranular` | [x] |
| `subtask list` | Standardized report format | `TestAlmostDone` | [x] |
| `subtask done` | Enforces git cleanliness | `TestSubtaskDoneFailedGranular` | [x] |
| `subtask done` | Performs auto-commit | `TestSubtaskDoneFailedGranular` | [x] |
| `validate` | Regression Check (fail > pass) | `TestTimestampRegression` | [x] |

## Automation Details

- [x] Detailed git error reporting (Stdout/Stderr captured and reported)
- [x] Subtask promotion recursion
- [x] Initial branch restoration in tests
